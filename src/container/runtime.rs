use crate::container::lifecycle;
use crate::tracking;
use colored::Colorize;
use indicatif::{ProgressBar, ProgressStyle};
use nix::unistd::{close, fork, pipe, read, write, dup2, ForkResult};
use nix::sched::{unshare, CloneFlags};
use nix::sys::signal::{self, Signal};
use std::os::unix::io::{RawFd, AsRawFd};
use std::fs::File;
use std::path::Path;
use std::process::Command;

extern "C" fn signal_handler(_: i32) {
    std::process::exit(0);
}

pub fn run_container(
    existing_name: Option<&str>,
    work_dir: &str,
    user_cmd: &[String],
    debug: bool,
    image: &str,
    ports: &str,
    isolated: bool,
    volumes: &[(String, String)],
) {
    if user_cmd.is_empty() {
        eprintln!("No command specified to launch in container.");
        return;
    }
    let container_id = match existing_name {
        Some(name) => name.to_string(),
        None => lifecycle::generate_container_id(),
    };
    let rootfs = crate::container::fs::get_rootfs(&container_id);
    if !Path::new(&rootfs).exists() {
        let pb = ProgressBar::new(4);
        pb.set_style(
            ProgressStyle::default_bar()
                .template("{spinner:.green} {msg}")
                .unwrap()
        );
        pb.set_message("Preparing container filesystem...");
        crate::container::fs::prepare_rootfs_dir(&container_id);
        pb.inc(1);
        pb.set_message("Extracting container image...");
        if let Err(e) = crate::container::image::extract_rootfs_tar(&container_id, image) {
            pb.finish_with_message("Extraction failed!");
            eprintln!(
                "{}",
                format!("Error: Invalid image provided ('{}'). Reason: {}", image, e)
                    .bright_red()
                    .bold()
            );
            crate::tracking::remove_container_from_tracking_by_name(&container_id);
            return;
        }
        pb.inc(1);
        pb.set_message("Copying working directory...");
        crate::container::fs::copy_directory_into_home(&container_id, work_dir);
        pb.inc(1);
        pb.set_message("Launching container...");
        pb.inc(1);
        pb.finish_with_message("Container build complete!");
    } else {
        println!("Container filesystem already exists. Skipping build.");
    }
    let (r, w) = pipe().expect("Failed to create pipe");
    match unsafe { fork() } {
        Ok(ForkResult::Parent { child: _child, .. }) => {
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
            println!("Use 'qube stop {}' or 'qube delete {}' to stop/delete it.\n", cpid, cpid);
            tracking::update_container_pid(&container_id, cpid, work_dir, user_cmd, image, ports, isolated);
        }
        Ok(ForkResult::Child) => {
            close(r).ok();
            child_container_process(w, &container_id, user_cmd, debug, image, ports, isolated, volumes);
        }
        Err(_e) => {
            eprintln!("Failed to fork()");
        }
    }
}

fn child_container_process(w: RawFd, cid: &str, cmd: &[String], debug: bool, _image: &str, _ports: &str, isolated: bool, volumes: &[(String, String)]) -> ! {
    let mut flags = CloneFlags::CLONE_NEWUTS | CloneFlags::CLONE_NEWNS;
    if isolated {
        flags |= CloneFlags::CLONE_NEWNET;
    }
    unshare(flags).unwrap();
    nix::unistd::sethostname("Qube").unwrap();
    crate::cgroup::setup_cgroup2();
    crate::container::fs::mount_proc(cid).unwrap();
    
    for (host_path, container_path) in volumes {
        println!("DEBUG: Attempting to mount {} -> {}", host_path, container_path);
        
        if let Err(e) = crate::container::fs::mount_volume(cid, host_path, container_path) {
            eprintln!("ERROR: Mount failed for {} -> {}: {:?}", host_path, container_path, e);
            std::process::exit(1);
        } else {
            println!("DEBUG: Successfully mounted {} -> {}", host_path, container_path);
        }
    
        println!("DEBUG: Checking mounts inside the container...");
        let output = Command::new("mount")
            .output()
            .expect("Failed to check mounted filesystems inside the container");
        println!("DEBUG: Container Mounts: \n{}", String::from_utf8_lossy(&output.stdout));
    }    
    
    std::env::set_current_dir(&crate::container::fs::get_rootfs(cid)).unwrap();
    nix::unistd::chroot(".").unwrap();
    nix::unistd::chdir("/home").unwrap();

    unsafe {
        signal::signal(Signal::SIGTERM, signal::SigHandler::Handler(signal_handler)).unwrap();
    }
    match unsafe { fork() } {
        Ok(ForkResult::Child) => {
            if !debug {
                detach_stdio();
            }
            launch_user_command(cmd);
        }
        Ok(ForkResult::Parent { child: _child, .. }) => {
            let child_pid = _child.as_raw();
            let _ = write(w, &child_pid.to_le_bytes());
            let _ = close(w);
            std::process::exit(0);
        }
        Err(_e) => {
            let _ = write(w, &(-1i32).to_le_bytes());
            std::process::exit(1);
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

    let mut command = Command::new("sh");
    command.arg("-c").arg(cmd_args.join(" "));

    match command.output() {
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
