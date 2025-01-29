use crate::cgroup;
use crate::tracking;
use colored::*;
use indicatif::{ProgressBar, ProgressStyle};
use nix::mount::{mount, MsFlags};
use nix::sched::{unshare, CloneFlags};
use nix::unistd::{chdir, chroot, execvp, sethostname};
use std::ffi::CString;
use std::fs;
use std::path::Path;
use std::process::Command;

pub const UBUNTU24_ROOTFS: &str = "/tmp/Qube_ubuntu24";
pub const UBUNTU24_TAR: &str = "ubuntu24rootfs.tar";

pub fn run_container(container_cmd: &[String]) {
    let total_steps = 5;
    let pb = ProgressBar::new(total_steps);
    pb.set_style(
        ProgressStyle::default_bar()
            .template("{spinner:.green} {msg} [{bar:40.white/blue}] {pos}/{len} ({eta})")
            .progress_chars(": "),
    );

    pb.set_message("Unsharing namespaces...");
    unshare(CloneFlags::CLONE_NEWUTS | CloneFlags::CLONE_NEWNS)
        .expect("Failed to unshare namespaces");
    sethostname("Qube").expect("Failed to set hostname");
    pb.inc(1);

    pb.set_message("Preparing container rootfs directory...");
    prepare_rootfs_dir();
    pb.inc(1);

    pb.set_message("Setting up the container");
    extract_rootfs_tar();
    pb.inc(1);

    pb.set_message("Configuring cgroup2...");
    let pid = cgroup::setup_cgroup2();
    tracking::track_container(pid);
    pb.inc(1);

    pb.set_message("Mounting /proc and chrooting...");
    mount_proc().expect("Failed to mount /proc");
    chdir(UBUNTU24_ROOTFS).expect("Failed to chdir to rootfs");
    chroot(".").expect("Failed to chroot to container rootfs");
    pb.inc(1);

    pb.finish_with_message("[+] Qube: Container ready!".truecolor(0, 200, 60).to_string());

    let cmd = CString::new(container_cmd[0].as_str()).unwrap();
    let mut cmd_args = vec![cmd.clone()];
    for arg in &container_cmd[1..] {
        cmd_args.push(CString::new(arg.as_str()).unwrap());
    }
    execvp(&cmd, &cmd_args).expect("Failed to exec command");

    println!("{}", "Container process exited (execvp failed).".bright_red());
}

fn prepare_rootfs_dir() {
    if Path::new(UBUNTU24_ROOTFS).exists() {
        fs::remove_dir_all(UBUNTU24_ROOTFS).ok();
    }
    fs::create_dir_all(UBUNTU24_ROOTFS).expect("Failed to create the UBUNTU24_ROOTFS directory");
}

fn extract_rootfs_tar() {
    if !Path::new(UBUNTU24_TAR).exists() {
        panic!(
            "{}",
            "ERROR: ubuntu24rootfs.tar not found! Please create or copy a real rootfs tar first."
                .bright_red()
        );
    }
    let status = Command::new("tar")
        .args(["-xf", UBUNTU24_TAR, "-C", UBUNTU24_ROOTFS])
        .status()
        .expect("Failed to spawn tar process");

    if !status.success() {
        panic!("{}", "Failed to extract the Ubuntu 24 rootfs! (tar error)".bright_red());
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
                let uptime = match tracking::get_process_uptime(pid_num) {
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
    let proc_path = format!("/proc/{}", pid);
    if !Path::new(&proc_path).exists() {
        println!("{}", format!("Container {} is already stopped or does not exist.", pid).bright_red());
        return;
    }

    use nix::sys::signal::{kill, Signal};
    use nix::unistd::Pid;

    if kill(Pid::from_raw(pid), Signal::SIGTERM).is_ok() {
        println!("Stopped container with PID: {}", pid);
        tracking::remove_container_from_tracking(pid);
    } else {
        println!("Failed to stop container with PID: {}", pid);
    }
}

pub fn kill_container(pid: i32) {
    let proc_path = format!("/proc/{}", pid);
    if !Path::new(&proc_path).exists() {
        println!("{}", format!("Container {} is already stopped or does not exist.", pid).bright_red());
        return;
    }

    use nix::sys::signal::{kill, Signal};
    use nix::unistd::Pid;

    if kill(Pid::from_raw(pid), Signal::SIGKILL).is_ok() {
        println!("Killed container with PID: {}", pid);
        tracking::remove_container_from_tracking(pid);
    } else {
        println!("Failed to kill container with PID: {}", pid);
    }
}
