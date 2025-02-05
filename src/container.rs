use crate::cgroup;
use crate::tracking;
use colored::Colorize;
use indicatif::{ProgressBar, ProgressStyle};
use libc::{fork, c_int};
use nix::mount::{mount, MsFlags};
use nix::sched::{unshare, CloneFlags};
use nix::sys::signal::{self, Signal};
use nix::unistd::{self, chdir, chroot, close, read, sethostname, write};
use rand::{distributions::Alphanumeric, Rng};
use std::fs;
use std::os::fd::RawFd;
use std::path::Path;
use std::process::Command;
use std::fs::File;
use nix::unistd::dup2;
use std::os::unix::io::AsRawFd;

pub const QUBE_CONTAINERS_BASE: &str = "/var/tmp/Qube_containers";
pub const UBUNTU24_TAR: &str = "/home/ewen/GitHub/Qube/ubuntu24rootfs_custom.tar";

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

pub fn run_container(existing_name: Option<&str>, work_dir: &str, user_cmd: &[String], debug: bool) {
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
    prepare_rootfs_dir(&container_id);
    pb.inc(1);
    pb.set_message("Extracting rootfs...");
    extract_rootfs_tar(&container_id);
    pb.inc(1);
    pb.set_message("Copying current directory contents -> /home/ ...");
    copy_directory_into_home(&container_id, work_dir);
    pb.inc(1);
    pb.finish_with_message("Container prepared".truecolor(0, 200, 60).to_string());
    let (r, w) = unistd::pipe().expect("Failed to create pipe");
    let f = unsafe { fork() };
    match f {
        -1 => eprintln!("Failed to fork()"),
        0 => {
            close(r).ok();
            child_container_process(w, &container_id, user_cmd, debug);
        }
        _pid => {
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
            tracking::update_container_pid(&container_id, cpid, work_dir, user_cmd);
        }
    }
}

fn child_container_process(w: RawFd, cid: &str, cmd: &[String], debug: bool) -> ! {
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
    mount_proc(&cid).unwrap();
    chdir(get_rootfs(&cid).as_str()).unwrap();
    chroot(".").unwrap();
    chdir("/home").unwrap();
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
            if !debug {
                detach_stdio();
            }
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

    let cwd = std::env::current_dir().unwrap_or_else(|e| {
        eprintln!("DEBUG: Failed to get current directory: {:?}", e);
        std::process::exit(1);
    });

    eprintln!("DEBUG: Running command in directory: {:?}", cwd);
    eprintln!("DEBUG: Running command: {:?}", cmd_args);

    let mut c = Command::new(&cmd_args[0]);
    if cmd_args.len() > 1 {
        c.args(&cmd_args[1..]);
    }

    match c.output() {
        Ok(output) => {
            eprintln!("DEBUG: Command exited with status: {:?}", output.status);
            if !output.stdout.is_empty() {
                eprintln!("DEBUG: Command stdout:\n{}", String::from_utf8_lossy(&output.stdout));
            }
            if !output.stderr.is_empty() {
                eprintln!("DEBUG: Command stderr:\n{}", String::from_utf8_lossy(&output.stderr));
            }
            std::process::exit(output.status.code().unwrap_or(1));
        }
        Err(e) => {
            eprintln!("DEBUG: Failed to run command: {:?}", e);
            std::process::exit(1);
        }
    }
}

fn prepare_rootfs_dir(cid: &str) {
    let rootfs = get_rootfs(cid);
    if Path::new(&rootfs).exists() {
        fs::remove_dir_all(&rootfs).ok();
    }
    fs::create_dir_all(&rootfs).unwrap();
}

fn extract_rootfs_tar(cid: &str) {
    let rootfs = get_rootfs(cid);
    if !Path::new(UBUNTU24_TAR).exists() {
        panic!("ERROR: {} not found!", UBUNTU24_TAR);
    }
    let s = Command::new("tar")
        .args(["-xf", UBUNTU24_TAR, "-C", &rootfs])
        .status()
        .unwrap();
    if !s.success() {
        panic!("Failed to extract the Ubuntu 24 rootfs!");
    }
}

fn copy_directory_into_home(cid: &str, wd: &str) {
    let hp = format!("{}/home", get_rootfs(cid));
    if !Path::new(&hp).exists() {
        fs::create_dir_all(&hp).ok();
    }

    let s = Command::new("cp")
        .args(["-r", &format!("{}/.", wd), &hp])
        .status()
        .unwrap();
    if !s.success() {
        eprintln!("Warning: copying {} -> {} failed.", wd, hp);
    }
}

fn mount_proc(cid: &str) -> Result<(), std::io::Error> {
    let p = format!("{}/proc", get_rootfs(cid));
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

fn get_rootfs(cid: &str) -> String {
    format!("{}/rootfs", format!("{}/{}", QUBE_CONTAINERS_BASE, cid))
}

fn detach_stdio() {
    let dev_null = File::open("/dev/null").unwrap_or_else(|e| {
        eprintln!("Failed to open /dev/null: {:?}", e);
        std::process::exit(1);
    });

    let fd = dev_null.as_raw_fd();

    for &fd_target in &[libc::STDIN_FILENO, libc::STDOUT_FILENO, libc::STDERR_FILENO] {
        if let Err(e) = dup2(fd, fd_target) {
            eprintln!("Failed to redirect fd {}: {:?}", fd_target, e);
            std::process::exit(1);
        }
    }
}

pub fn list_containers() {
    let e = tracking::get_all_tracked_entries();
    if e.is_empty() {
        println!("{}", "No containers tracked.".red().bold());
        return;
    }
    println!("╔════════════════════╦════════════╦═══════════╦══════════════╗");
    println!("{}", format!(
        "| {:<18} | {:<10} | {:<9} | {:<12} |",
        "NAME".bold().truecolor(255, 165, 0),
        "PID".bold().truecolor(0, 200, 60),
        "UPTIME".bold().truecolor(150, 200, 150),
        "STATUS".bold().truecolor(150, 200, 150)
    ));
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
