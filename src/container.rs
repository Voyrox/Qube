use crate::cgroup;
use crate::tracking;
use colored::*;
use indicatif::{ProgressBar, ProgressStyle};
use libc::fork;
use nix::mount::{mount, MsFlags};
use nix::sched::{unshare, CloneFlags};
use nix::unistd::{self, chdir, chroot, close, read, sethostname, write};
use std::os::fd::RawFd;
use std::fs;
use std::path::Path;
use std::process::Command;

pub const UBUNTU24_ROOTFS: &str = "/tmp/Qube_ubuntu24";
pub const UBUNTU24_TAR: &str = "ubuntu24rootfs.tar";

pub fn run_container(_user_cmd: &[String]) {
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

    let (pipe_rd, pipe_wr) = unistd::pipe().expect("Failed to create pipe");

    let fork_result = unsafe { fork() };
    match fork_result {
        -1 => {
            eprintln!("Failed to fork()");
        }
        0 => {
            close(pipe_rd).ok();
            child_container_process(pipe_wr);
        }
        _child_pid => {
            close(pipe_wr).ok();
            let mut buf = [0u8; 4];
            let n = read(pipe_rd, &mut buf).unwrap_or(0);
            close(pipe_rd).ok();

            if n < 4 {
                eprintln!("Container process did not report a final PID (it may have exited early).");
                return;
            }

            let container_pid = i32::from_le_bytes(buf);

            println!("\nContainer launched with PID: {container_pid}");
            println!("Use 'qube stop {container_pid}' or 'qube kill {container_pid}' to stop/kill it.\n");
        }
    }
}

fn child_container_process(pipefd: RawFd) -> ! {
    let total_steps_child = 3;
    let pb_child = ProgressBar::new(total_steps_child);
    pb_child.set_style(
        ProgressStyle::default_bar()
            .template("{spinner:.green} {msg} [{bar:40.white/blue}] {pos}/{len} ({eta})")
            .progress_chars("=>-"),
    );

    pb_child.set_message("Unsharing namespaces...");
    unshare(CloneFlags::CLONE_NEWUTS | CloneFlags::CLONE_NEWNS)
        .expect("Failed to unshare namespaces");
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

    match unsafe { fork() } {
        -1 => {
            eprintln!("Failed second fork()");
            let _ = write(pipefd, &(-1i32).to_le_bytes());
            std::process::exit(1);
        }
        0 => {
            container_init_loop();
        }
        grandchild_pid => {
            tracking::track_container(grandchild_pid);
            let _ = write(pipefd, &grandchild_pid.to_le_bytes());
            let _ = close(pipefd);
            std::process::exit(0);
        }
    }
}

fn container_init_loop() -> ! {
    use nix::sys::wait::{waitpid, WaitPidFlag, WaitStatus};
    loop {
        match waitpid(None, Some(WaitPidFlag::WNOHANG)) {
            Ok(WaitStatus::StillAlive)
            | Ok(WaitStatus::Exited(_, _))
            | Ok(WaitStatus::Signaled(_, _, _))
            | Ok(WaitStatus::Stopped(_, _))
            | Ok(WaitStatus::Continued(_)) => {
                std::thread::sleep(std::time::Duration::from_secs(5));
            }
            Err(nix::errno::Errno::ECHILD) => {
                std::thread::sleep(std::time::Duration::from_secs(5));
            }
            _ => {
                std::thread::sleep(std::time::Duration::from_secs(5));
            }
        }
    }
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

        println!("{}", "╔═════════════════╦════════════╦══════════════╗");
        println!(
            "{}",
            format!(
                "| {:<15} | {:<10} | {:<12} |",
                "CONTAINER ID".bold().truecolor(255, 165, 0),
                "STATUS".bold().truecolor(0, 200, 60),
                "UPTIME".bold().truecolor(150, 200, 150)
            )
        );
        println!("{}", "╠═════════════════╬════════════╬══════════════╣");

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

        println!("{}", "╚═════════════════╩════════════╩══════════════╝");

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
