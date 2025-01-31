use crate::cgroup;
use crate::tracking;
use colored::Colorize;
use indicatif::{ProgressBar, ProgressStyle};
use libc::{fork, c_int};
use nix::mount::{mount, MsFlags};
use nix::sched::{unshare, CloneFlags};
use nix::sys::signal::{self, Signal};
use nix::unistd::{self, chdir, chroot, close, read, sethostname, write, setsid};
use rand::{distributions::Alphanumeric, Rng};
use std::fs;
use std::os::fd::RawFd;
use std::os::unix::process::CommandExt;
use std::path::Path;
use std::process::Command;

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

extern "C" fn signal_handler(_: c_int) {
    std::process::exit(0);
}

pub fn run_container(existing_name: Option<&str>, work_dir: &str, user_cmd: &[String]) {
    if user_cmd.is_empty() {
        eprintln!("No command specified to launch in container.");
        return;
    }
    let container_id = match existing_name {
        Some(x) => x.to_string(),
        None => generate_container_id(),
    };
    let p = 3;
    let pb = ProgressBar::new(p);
    pb.set_style(
        ProgressStyle::default_bar()
            .template("{spinner:.green} {msg} [{bar:40.white/blue}] {pos}/{len} ({eta})")
            .progress_chars("█-"),
    );
    pb.set_message("Preparing container rootfs directory...");
    prepare_rootfs_dir();
    pb.inc(1);
    pb.set_message("Extracting rootfs...");
    extract_rootfs_tar();
    pb.inc(1);
    pb.set_message("Copying user directory -> /var/www/ ...");
    copy_directory_into_www(work_dir);
    pb.inc(1);
    pb.finish_with_message("Container prepared".truecolor(0, 200, 60).to_string());
    let (r, w) = unistd::pipe().expect("Failed to create pipe");
    let f = unsafe { fork() };
    match f {
        -1 => eprintln!("Failed to fork()"),
        0 => {
            close(r).ok();
            child_container_process(w, &container_id, user_cmd);
        }
        pid => {
            close(w).ok();
            let mut buf = [0u8; 4];
            let n = read(r, &mut buf).unwrap_or(0);
            close(r).ok();
            if n < 4 {
                eprintln!("Container process did not report a final PID (it may have exited).");
                return;
            }
            let cpid = i32::from_le_bytes(buf);
            println!("\nContainer launched with ID: {} (PID: {})", container_id, cpid);
            println!("Use 'qube stop {}' or 'qube kill {}' to stop/kill it.\n", cpid, cpid);
            tracking::track_container_named(&container_id, cpid, work_dir, user_cmd.to_vec());
        }
    }
}

fn child_container_process(w: RawFd, cid: &str, cmd: &[String]) -> ! {
    let p = 3;
    let pb = ProgressBar::new(p);
    pb.set_style(
        ProgressStyle::default_bar()
            .template("{spinner:.green} {msg} [{bar:40.white/blue}] {pos}/{len} ({eta})")
            .progress_chars("=>-"),
    );
    pb.set_message("Unsharing namespaces...");
    unshare(CloneFlags::CLONE_NEWUTS | CloneFlags::CLONE_NEWNS).unwrap();
    sethostname("Qube").unwrap();
    pb.inc(1);
    pb.set_message("Setting up cgroup...");
    cgroup::setup_cgroup2();
    pb.inc(1);
    pb.set_message("Mounting proc & chroot...");
    mount_proc().unwrap();
    chdir(UBUNTU24_ROOTFS).unwrap();
    chroot(".").unwrap();
    pb.inc(1);
    pb.finish_with_message("Container environment set up.");
    unsafe {
        signal::signal(Signal::SIGTERM, signal::SigHandler::Handler(signal_handler)).unwrap();
    }
    match unsafe { fork() } {
        -1 => {
            let _ = write(w, &(-1i32).to_le_bytes());
            std::process::exit(1);
        }
        0 => {
            launch_user_command(cmd);
        }
        gpid => {
            let _ = write(w, &gpid.to_le_bytes());
            let _ = close(w);
            std::process::exit(0);
        }
    }
}

fn launch_user_command(cmd_args: &[String]) -> ! {
    if cmd_args.is_empty() {
        eprintln!("No command specified to launch in container.");
        std::process::exit(1);
    }
    setsid().ok();
    let mut c = Command::new(&cmd_args[0]);
    if cmd_args.len() > 1 {
        c.args(&cmd_args[1..]);
    }
    let e = c.exec();
    eprintln!("Failed to exec user command in container: {:?}", e);
    std::process::exit(1);
}

fn prepare_rootfs_dir() {
    if Path::new(UBUNTU24_ROOTFS).exists() {
        fs::remove_dir_all(UBUNTU24_ROOTFS).ok();
    }
    fs::create_dir_all(UBUNTU24_ROOTFS).unwrap();
}

fn extract_rootfs_tar() {
    if !Path::new(UBUNTU24_TAR).exists() {
        panic!("ERROR: {} not found!", UBUNTU24_TAR);
    }
    let s = Command::new("tar")
        .args(["-xf", UBUNTU24_TAR, "-C", UBUNTU24_ROOTFS])
        .status()
        .unwrap();
    if !s.success() {
        panic!("Failed to extract the Ubuntu 24 rootfs!");
    }
}

fn copy_directory_into_www(wd: &str) {
    let wp = format!("{}/var/www", UBUNTU24_ROOTFS);
    if !Path::new(&wp).exists() {
        fs::create_dir_all(&wp).ok();
    }
    let s = Command::new("cp")
        .args(["-r", wd, &wp])
        .status()
        .unwrap();
    if !s.success() {
        eprintln!("Warning: copying {} -> {} failed.", wd, wp);
    }
}

fn mount_proc() -> Result<(), std::io::Error> {
    let p = format!("{}/proc", UBUNTU24_ROOTFS);
    fs::create_dir_all(&p)?;
    mount(
        Some("proc"),
        p.as_str(),
        Some("proc"),
        MsFlags::MS_NOEXEC | MsFlags::MS_NOSUID | MsFlags::MS_NODEV,
        None::<&str>,
    )
    .map_err(|e| std::io::Error::new(std::io::ErrorKind::Other, e))
}

pub fn list_containers() {
    let e = tracking::get_all_tracked_entries();
    if e.is_empty() {
        println!("{}", "No containers tracked.".red().bold());
        return;
    }
    println!("╔════════════════════╦════════════╦═══════════╦══════════════╗");
    println!("{}", format!("| {:<18} | {:<10} | {:<9} | {:<12} |","NAME".bold().truecolor(255, 165, 0),"PID".bold().truecolor(0, 200, 60),"UPTIME".bold().truecolor(150, 200, 150),"STATUS".bold().truecolor(150, 200, 150)));
    println!("╠════════════════════╬════════════╬═══════════╬══════════════╣");
    for x in e {
        let path = format!("/proc/{}", x.pid);
        if Path::new(&path).exists() {
            match tracking::get_process_uptime(x.pid) {
                Ok(u) => println!("║ {:<18} ║ {:<10} ║ {:<9} ║ {:<12} ║", x.name, x.pid, u, "RUNNING"),
                Err(_) => println!("║ {:<18} ║ {:<10} ║ {:<9} ║ {:<12} ║", x.name, x.pid, "N/A", "RUNNING"),
            }
        } else {
            println!("║ {:<18} ║ {:<10} ║ {:<9} ║ {:<12} ║", x.name, x.pid, 0, "EXITED");
        }
    }
    println!("╚════════════════════╩════════════╩═══════════╩══════════════╝");
}

pub fn stop_container(pid: i32) {
    use nix::sys::signal::{kill, Signal};
    use nix::unistd::Pid;
    let p = format!("/proc/{}", pid);
    if !Path::new(&p).exists() {
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
    let p = format!("/proc/{}", pid);
    if !Path::new(&p).exists() {
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
