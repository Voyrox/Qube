use crate::cgroup;
use crate::tracking;
use colored::*;
use indicatif::{ProgressBar, ProgressStyle};
use libc::fork;
use nix::mount::{mount, MsFlags};
use nix::sched::{unshare, CloneFlags};
use nix::unistd::{chdir, chroot, execvp, getpid, sethostname};
use std::ffi::CString;
use std::fs;
use std::path::Path;
use std::process::Command;

pub const UBUNTU24_ROOTFS: &str = "/tmp/Qube_ubuntu24";
pub const UBUNTU24_TAR: &str = "ubuntu24rootfs.tar";

pub fn run_container(user_cmd: &[String]) {
    let total_steps_parent = 2;
    let pb_parent = ProgressBar::new(total_steps_parent);
    pb_parent.set_style(
        ProgressStyle::default_bar()
            .template("{spinner:.green} {msg} [{bar:40.white/blue}] {pos}/{len} ({eta})")
            .progress_chars("=>-"),
    );

    pb_parent.set_message("Preparing container rootfs directory...");
    prepare_rootfs_dir();
    pb_parent.inc(1);

    pb_parent.set_message("Extracting rootfs...");
    extract_rootfs_tar();
    pb_parent.inc(1);

    pb_parent.finish_with_message("Container prepared".truecolor(0, 200, 60).to_string());

    let fork_result = unsafe { fork() };
    match fork_result {
        -1 => {
            eprintln!("Failed to fork()");
        }
        0 => {
            child_container_process(user_cmd);
        }
        child_pid => {
            println!(
                "\nContainer launched with PID: {}",
                child_pid
            );
            println!("Use 'qube stop {pid}' or 'qube kill {pid}' to stop/kill it.\n", pid = child_pid);
        }
    }
}

fn child_container_process(user_cmd: &[String]) -> ! {
    let total_steps_child = 3;
    let pb_child = ProgressBar::new(total_steps_child);
    pb_child.set_style(
        ProgressStyle::default_bar()
            .template("{spinner:.green} {msg} [{bar:40.white/blue}] {pos}/{len} ({eta})")
            .progress_chars("=>-"),
    );

    unshare(CloneFlags::CLONE_NEWUTS | CloneFlags::CLONE_NEWNS)
        .expect("Failed to unshare namespaces");
    sethostname("Qube").expect("Failed to set hostname");

    let pid = getpid().as_raw();
    cgroup::setup_cgroup2();
    tracking::track_container(pid);

    mount_proc().expect("Failed to mount /proc");
    chdir(UBUNTU24_ROOTFS).expect("Failed to chdir to rootfs");
    chroot(".").expect("Failed to chroot to container rootfs");

    let user_str = user_cmd.join(" ");
    let script = format!("{}; exec sleep infinity", user_str);
    let final_cmd = vec!["/bin/sh", "-c", &script];

    let cmd = CString::new(final_cmd[0]).unwrap();
    let cmd_args: Vec<CString> = final_cmd
        .iter()
        .map(|s| CString::new(*s).unwrap())
        .collect();

    execvp(&cmd, &cmd_args).expect("Failed to exec command");

    eprintln!("Container child process: execvp failed.");
    std::process::exit(1);
}

fn prepare_rootfs_dir() {
    if Path::new(UBUNTU24_ROOTFS).exists() {
        fs::remove_dir_all(UBUNTU24_ROOTFS).ok();
    }
    fs::create_dir_all(UBUNTU24_ROOTFS).expect("Failed to create UBUNTU24_ROOTFS directory");
}

fn extract_rootfs_tar() {
    if !Path::new(UBUNTU24_TAR).exists() {
        panic!("ERROR: ubuntu24rootfs.tar not found! Please provide a valid rootfs tar.");
    }
    let status = Command::new("tar")
        .args(["-xf", UBUNTU24_TAR, "-C", UBUNTU24_ROOTFS])
        .status()
        .expect("Failed to spawn tar process");
    if !status.success() {
        panic!("Failed to extract the Ubuntu 24 rootfs! (tar error)");
    }
}

fn mount_proc() -> Result<(), std::io::Error> {
    let proc_path = format!("{}/proc", UBUNTU24_ROOTFS);
    fs::create_dir_all(&proc_path)?;
    mount(
        Some("proc"),
        proc_path.as_str(),
        Some("proc"),
        MsFlags::MS_NOEXEC | MsFlags::MS_NOSUID | MsFlags::MS_NODEV,
        None::<&str>,
    )
    .map_err(|e| std::io::Error::new(std::io::ErrorKind::Other, e))?;
    Ok(())
}

pub fn list_containers() {
    if let Ok(contents) = fs::read_to_string(tracking::CONTAINER_LIST_FILE) {
        let mut valid_pids = Vec::new();

        println!("{}", "╔═════════════════╦════════════╦══════════════╗".truecolor(247, 76, 0));
        println!(
            "{}",
            format!(
                "║ {:<15} ║ {:<10} ║ {:<12} ║",
                "CONTAINER ID".bold().truecolor(255, 165, 0),
                "STATUS".bold().truecolor(200, 150, 100),
                "UPTIME".bold().truecolor(150, 200, 150)
            )
        );
        println!("{}", "╠═════════════════╬════════════╬══════════════╣".truecolor(247, 76, 0));

        for pid_str in contents.lines() {
            let pid_num = pid_str.parse::<i32>().unwrap_or(0);
            let proc_path = format!("/proc/{}", pid_num);

            if Path::new(&proc_path).exists() {
                let uptime = match crate::tracking::get_process_uptime(pid_num) {
                    Ok(t) => format!("{}s", t),
                    Err(_) => "N/A".to_string(),
                };
                println!("║ {:<15} ║ {:<10} ║ {:<12} ║", pid_str, "RUNNING", uptime);
                valid_pids.push(pid_str.to_string());
            }
        }

        println!("{}", "╚═════════════════╩════════════╩══════════════╝".truecolor(247, 76, 0));

        fs::write(tracking::CONTAINER_LIST_FILE, valid_pids.join("\n"))
            .expect("Failed to update container list");
    } else {
        println!("{}", "No running containers.".bright_red().bold());
    }
}

pub fn stop_container(pid: i32) {
    use nix::sys::signal::{kill, Signal};
    use nix::unistd::Pid;

    let proc_path = format!("/proc/{}", pid);
    if !Path::new(&proc_path).exists() {
        println!("Container {} is already stopped or doesn't exist.", pid);
        return;
    }

    if kill(Pid::from_raw(pid), Signal::SIGTERM).is_ok() {
        println!("Stopped container with PID: {}", pid);
        tracking::remove_container_from_tracking(pid);
    } else {
        println!("Failed to stop container with PID: {}", pid);
    }
}

pub fn kill_container(pid: i32) {
    use nix::sys::signal::{kill, Signal};
    use nix::unistd::Pid;

    let proc_path = format!("/proc/{}", pid);
    if !Path::new(&proc_path).exists() {
        println!("Container {} is already stopped or doesn't exist.", pid);
        return;
    }

    if kill(Pid::from_raw(pid), Signal::SIGKILL).is_ok() {
        println!("Killed container with PID: {}", pid);
        tracking::remove_container_from_tracking(pid);
    } else {
        println!("Failed to kill container with PID: {}", pid);
    }
}
