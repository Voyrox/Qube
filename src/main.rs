use nix::sched::{unshare, CloneFlags};
use nix::unistd::{chdir, chroot, execvp, sethostname, getpid, Pid};
use nix::mount::{mount, umount, MsFlags};
use nix::sys::signal::{kill, Signal};
use colored::*;
use std::ffi::CString;
use std::fs;
use std::os::unix::fs::PermissionsExt;
use std::path::Path;
use std::process::{Command, exit};
use std::collections::HashMap;
use std::io::{self, Write};

const UBUNTU24_ROOTFS: &str = "/tmp/Qube_ubuntu24";
const UBUNTU24_TAR: &str     = "ubuntu24rootfs.tar";
const CGROUP_ROOT: &str = "/sys/fs/cgroup/QubeContainers";
const TRACKING_DIR: &str = "/var/lib/Qube";
const CONTAINER_LIST_FILE: &str = "/var/lib/Qube/containers.txt";

const MEMORY_MAX: &str    = "2147483648";  // 2GB
const MEMORY_SWAP_MAX: &str = "1073741824"; // 1GB

fn main() {
    if nix::unistd::geteuid().as_raw() != 0 {
        eprintln!("{}", "Error: This program must be run as root (use sudo).".bright_red().bold());
        exit(1);
    }

    let args: Vec<String> = std::env::args().collect();
    if args.len() < 2 {
        eprintln!("{}", "Usage: qube <run|list|stop|kill> [args...]".bright_red());
        exit(1);
    }

    match args[1].as_str() {
        "run" => {
            let container_cmd: Vec<String> = args[2..].to_vec();
            if container_cmd.is_empty() {
                eprintln!("{}", "Please specify a command to run inside the container, e.g. /bin/bash".bright_red());
                exit(1);
            }
            run_container(&container_cmd);
        }
        "list" => list_containers(),
        "stop" => {
            if args.len() < 3 {
                eprintln!("{}", "Usage: qube stop <pid>".bright_red());
                exit(1);
            }
            stop_container(args[2].parse().unwrap());
        }
        "kill" => {
            if args.len() < 3 {
                eprintln!("{}", "Usage: qube kill <pid>".bright_red());
                exit(1);
            }
            kill_container(args[2].parse().unwrap());
        }
        _ => {
            eprintln!("{}", format!("Unknown subcommand: {}", args[1]).bright_red());
            exit(1);
        }
    }
}

fn run_container(container_cmd: &[String]) {
    println!("{}", "[+] Qube: Starting container with REAL Ubuntu 24.04 rootfs".truecolor(247, 76, 0));

    unshare(CloneFlags::CLONE_NEWUTS | CloneFlags::CLONE_NEWNS)
        .expect("Failed to unshare namespaces");

    sethostname("Qube").expect("Failed to set hostname");

    prepare_rootfs();

    let pid = setup_cgroup2();
    track_container(pid);

    let proc_path = format!("{}/proc", UBUNTU24_ROOTFS);
    fs::create_dir_all(&proc_path).ok();
    mount(
        Some("proc"),
        proc_path.as_str(),
        Some("proc"),
        MsFlags::MS_NOEXEC | MsFlags::MS_NOSUID | MsFlags::MS_NODEV,
        None::<&str>,
    ).expect("Failed to mount /proc");

    chdir(UBUNTU24_ROOTFS).expect("Failed to chdir to rootfs");
    chroot(".").expect("Failed to chroot to container rootfs");

    let cmd = CString::new(container_cmd[0].as_str()).unwrap();
    let mut cmd_args = vec![cmd.clone()];
    for arg in &container_cmd[1..] {
        cmd_args.push(CString::new(arg.as_str()).unwrap());
    }

    execvp(&cmd, &cmd_args).expect("Failed to exec command");

    umount(proc_path.as_str()).ok();
    println!("{}", "Container process exited (execvp failed).".bright_red());
}

fn prepare_rootfs() {
    if Path::new(UBUNTU24_ROOTFS).exists() {
        fs::remove_dir_all(UBUNTU24_ROOTFS).ok();
    }
    fs::create_dir_all(UBUNTU24_ROOTFS).unwrap();

    if !Path::new(UBUNTU24_TAR).exists() {
        panic!("{}", "ERROR: ubuntu24rootfs.tar not found! Please create or copy a real rootfs tar first.".bright_red());
    }

    println!("{}", "[+] Extracting ubuntu24rootfs.tar to /tmp/Qube_ubuntu24".truecolor(247, 76, 0));
    let status = Command::new("tar")
        .args(["-xf", UBUNTU24_TAR, "-C", UBUNTU24_ROOTFS])
        .status()
        .expect("Failed to spawn `tar` process");

    if !status.success() {
        panic!("{}", "Failed to extract the real Ubuntu 24 rootfs! (tar error)".bright_red());
    }
}

fn setup_cgroup2() -> i32 {
    let pid = getpid().as_raw();
    let cgroup_path = format!("{}/{}", CGROUP_ROOT, pid);

    fs::create_dir_all(&cgroup_path).expect("Failed to create cgroup dir");
    fs::set_permissions(&cgroup_path, fs::Permissions::from_mode(0o755))
        .expect("Failed to set permissions on cgroup directory");

    let mem_max_path = format!("{}/memory.max", cgroup_path);
    let mem_swap_path = format!("{}/memory.swap.max", cgroup_path);

    if let Err(e) = fs::write(&mem_max_path, MEMORY_MAX) {
        eprintln!("Warning: Failed to set memory.max: {}", e);
    }    
    if let Err(e) = fs::write(&mem_swap_path, MEMORY_SWAP_MAX) {
        eprintln!("Warning: Failed to set memory.swap.max: {}", e);
    }    

    let cgroup_procs = format!("{}/cgroup.procs", cgroup_path);
    fs::write(&cgroup_procs, pid.to_string()).expect("Failed to write PID to cgroup.procs");

    pid
}

fn track_container(pid: i32) {
    fs::create_dir_all(TRACKING_DIR).expect("Failed to create tracking directory");

    let mut file = fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(CONTAINER_LIST_FILE)
        .expect("Failed to open container tracking file");

    writeln!(file, "{}", pid).expect("Failed to write PID to tracking file");
}

fn list_containers() {
    if let Ok(contents) = fs::read_to_string(CONTAINER_LIST_FILE) {
        let pids: Vec<&str> = contents.lines().collect();
        if pids.is_empty() {
            println!("{}", "No running containers.".bright_red().bold());
        } else {
            println!("{}", "╔═════════════════╦════════════╗".truecolor(247, 76, 0));
            println!(
                "{}",
                format!(
                    "║ {:<15} ║ {:<10} ║",
                    "CONTAINER ID".bold().truecolor(255, 165, 0),
                    "STATUS".bold().truecolor(200, 150, 100)
                )
            );
            println!("{}", "╠═════════════════╬════════════╣".truecolor(247, 76, 0));

            for pid in pids {
                println!(
                    "║ {:<15} ║ {:<10} ║",
                    pid.truecolor(255, 165, 0),
                    "RUNNING".truecolor(200, 150, 100)
                );
            }
            println!("{}", "╚═════════════════╩════════════╝".truecolor(247, 76, 0));
        }
    } else {
        println!("{}", "No running containers.".bright_red().bold());
    }
}

fn stop_container(pid: i32) {
    if kill(Pid::from_raw(pid), Signal::SIGTERM).is_ok() {
        println!("Stopped container with PID: {}", pid);
    } else {
        println!("Failed to stop container with PID: {}", pid);
    }
}

fn kill_container(pid: i32) {
    if kill(Pid::from_raw(pid), Signal::SIGKILL).is_ok() {
        println!("Killed container with PID: {}", pid);
    } else {
        println!("Failed to kill container with PID: {}", pid);
    }
}
