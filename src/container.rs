use crate::cgroup;
use crate::tracking;
use colored::Colorize;
use indicatif::{ProgressBar, ProgressStyle};
use libc::{fork, c_int};
use nix::mount::{mount, MsFlags};
use nix::sched::{unshare, CloneFlags};
use nix::unistd::{self, chdir, chroot, close, read, sethostname, write};
use rand::{distributions::Alphanumeric, Rng};
use std::fs;
use std::os::fd::RawFd;
use std::path::Path;
use std::process::Command;
use nix::sys::signal::{Signal, self};
use std::os::unix::process::CommandExt;


pub const UBUNTU24_ROOTFS: &str = "/tmp/Qube_ubuntu24";
pub const UBUNTU24_TAR: &str = "ubuntu24rootfs_custom.tar";

fn generate_container_id() -> String {
    let rand_str: String = rand::thread_rng()
        .sample_iter(&Alphanumeric)
        .take(6)
        .map(char::from)
        .collect();
    format!("Qube-{}", rand_str)
}

extern "C" fn signal_handler(_sig: c_int) {
    println!("Received SIGTERM inside container's init process, stopping container...");
    std::process::exit(0);
}

pub fn run_container(user_cmd: &[String]) {
    let total_steps_parent = 2;
    let pb_parent = ProgressBar::new(total_steps_parent);
    pb_parent.set_style(
        ProgressStyle::default_bar()
            .template("{spinner:.green} {msg} [{bar:40.white/blue}] {pos}/{len} ({eta})")
            .progress_chars("█-"),
    );

    pb_parent.set_message("Preparing container rootfs directory...");
    prepare_rootfs_dir();
    pb_parent.inc(1);

    pb_parent.set_message("Extracting rootfs...");
    extract_rootfs_tar();
    pb_parent.inc(1);

    pb_parent.finish_with_message("Container prepared".truecolor(0, 200, 60).to_string());

    let container_id = generate_container_id();
    let (pipe_rd, pipe_wr) = unistd::pipe().expect("Failed to create pipe");
    let fork_result = unsafe { fork() };

    match fork_result {
        -1 => eprintln!("Failed to fork()"),
        0 => {
            close(pipe_rd).ok();
            child_container_process(pipe_wr, &container_id, user_cmd);
        }
        pid => {
            close(pipe_wr).ok();
            let mut buf = [0u8; 4];
            let n = read(pipe_rd, &mut buf).unwrap_or(0);
            close(pipe_rd).ok();
            if n < 4 {
                eprintln!("Container process did not report a final PID (it may have exited).");
                return;
            }
            let container_pid = i32::from_le_bytes(buf);
            println!(
                "\nContainer launched with ID: {} (PID: {})",
                container_id, container_pid
            );
            println!(
                "Use 'qube stop {}' or 'qube kill {}' to stop/kill it.\n",
                container_pid, container_pid
            );

            tracking::track_container_named(
                &container_id,
                container_pid,
                user_cmd.to_vec()
            );
        }
    }
}

fn child_container_process(pipefd: RawFd, container_id: &str, user_cmd: &[String]) -> ! {
    let total_steps_child = 3;
    let pb_child = ProgressBar::new(total_steps_child);
    pb_child.set_style(
        ProgressStyle::default_bar()
            .template("{spinner:.green} {msg} [{bar:40.white/blue}] {pos}/{len} ({eta})")
            .progress_chars("=>-"),
    );

    pb_child.set_message("Unsharing namespaces...");
    unshare(CloneFlags::CLONE_NEWUTS | CloneFlags::CLONE_NEWNS).expect("Failed to unshare namespaces");
    sethostname("Qube").expect("Failed to set hostname");
    pb_child.inc(1);

    pb_child.set_message("Setting up cgroup...");
    cgroup::setup_cgroup2();
    pb_child.inc(1);

    pb_child.set_message("Mounting proc & chroot...");
    mount_proc().expect("Failed to mount /proc");
    chdir(UBUNTU24_ROOTFS).expect("Failed to chdir to rootfs");
    chroot(".").expect("Failed to chroot to container rootfs");
    pb_child.inc(1);

    pb_child.finish_with_message("Container environment set up.");

    unsafe {
        signal::signal(Signal::SIGTERM, signal::SigHandler::Handler(signal_handler))
            .expect("Failed to register signal handler");
    }

    match unsafe { fork() } {
        -1 => {
            let _ = write(pipefd, &(-1i32).to_le_bytes());
            std::process::exit(1);
        }
        0 => {
            launch_user_command(user_cmd);
        }
        grandchild_pid => {
            let _ = write(pipefd, &grandchild_pid.to_le_bytes());
            let _ = close(pipefd);
            std::process::exit(0);
        }
    }
}

fn launch_user_command(user_cmd: &[String]) -> ! {

    if user_cmd.is_empty() {
        eprintln!("No command specified to launch in container.");
        std::process::exit(1);
    }

    let mut cmd = Command::new(&user_cmd[0]);
    if user_cmd.len() > 1 {
        cmd.args(&user_cmd[1..]);
    }

    let err = cmd.exec();
    eprintln!("Failed to exec user command in container: {:?}", err);
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
        panic!("ERROR: {} not found!", UBUNTU24_TAR);
    }
    let status = Command::new("tar")
        .args(["-xf", UBUNTU24_TAR, "-C", UBUNTU24_ROOTFS])
        .status()
        .expect("Failed to spawn tar process");
    if !status.success() {
        panic!("Failed to extract the Ubuntu 24 rootfs!");
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
    let running_pids = tracking::get_running_containers();

    if running_pids.is_empty() {
        println!("{}", "No running containers.".red().bold());
        return;
    }

    println!("╔═════════════════╦════════════╦══════════════╗");
    println!(
        "{}",
        format!(
            "| {:<15} | {:<10} | {:<12} |",
            "PID".bold().truecolor(255, 165, 0),
            "STATUS".bold().truecolor(0, 200, 60),
            "UPTIME".bold().truecolor(150, 200, 150)
        )
    );
    println!("╠═════════════════╬════════════╬══════════════╣");

    for pid in running_pids {
        let proc_path = format!("/proc/{}", pid);
        if Path::new(&proc_path).exists() {
            let uptime = match tracking::get_process_uptime(pid) {
                Ok(t) => format!("{}s", t),
                Err(_) => "N/A".to_string(),
            };
            println!("║ {:<15} ║ {:<10} ║ {:<12} ║", pid, "RUNNING", uptime);
        }
    }
    println!("╚═════════════════╩════════════╩══════════════╝");
}

pub fn stop_container(pid: i32) {
    use nix::sys::signal::{kill, Signal};
    use nix::unistd::Pid;
    let path = format!("/proc/{}", pid);
    if !Path::new(&path).exists() {
        println!("Container {} is not running.", pid);
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
    let path = format!("/proc/{}", pid);
    if !Path::new(&path).exists() {
        println!("Container {} is not running.", pid);
        return;
    }
    if kill(Pid::from_raw(pid), Signal::SIGKILL).is_ok() {
        println!("Killed container with PID: {}", pid);
        tracking::remove_container_from_tracking(pid);
    } else {
        println!("Failed to kill container with PID: {}", pid);
    }
}
