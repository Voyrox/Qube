use crate::container::{fs, image};
use crate::tracking;
use colored::Colorize;
use indicatif::{ProgressBar, ProgressStyle};
use rand::{distributions::Alphanumeric, Rng};
use std::path::Path;
use nix::sys::signal::{kill, Signal};
use nix::unistd::Pid;

pub fn generate_container_id() -> String {
    let rand_str: String = rand::thread_rng()
        .sample_iter(&Alphanumeric)
        .take(6)
        .map(char::from)
        .collect();
    format!("Qube-{}", rand_str)
}

pub fn build_container(existing_name: Option<&str>, work_dir: &str, image: &str) -> String {
    let container_id = match existing_name {
        Some(name) => name.to_string(),
        None => generate_container_id(),
    };
    let rootfs = fs::get_rootfs(&container_id);
    if !Path::new(&rootfs).exists() {
        let pb = ProgressBar::new(4);
        pb.set_style(
            ProgressStyle::default_bar()
                .template("{spinner:.green} [{elapsed_precise}] [{bar:40.cyan/red}] {bytes}/{total_bytes} ({eta}, {percent}%) Building container")
                .unwrap()
                .progress_chars("∷∷"),
        );
        pb.set_message("Preparing container filesystem...");
        fs::prepare_rootfs_dir(&container_id);
        pb.inc(1);
        pb.set_message("Extracting container image...");
        if let Err(e) = image::extract_rootfs_tar(&container_id, image) {
            pb.finish_with_message("Extraction failed!");
            eprintln!(
                "{}",
                format!("Error: Invalid image provided ('{}'). Reason: {}", image, e)
                    .bright_red()
                    .bold()
            );
            crate::tracking::remove_container_from_tracking_by_name(&container_id);
            return container_id;
        }
        pb.inc(1);
        pb.set_message("Copying working directory...");
        fs::copy_directory_into_home(&container_id, work_dir);
        pb.inc(1);
        pb.set_message("Container build complete!");
        pb.inc(1);
        pb.finish_with_message("Container build complete!");
    } else {
        println!("Container filesystem already exists. Skipping build.");
    }
    container_id
}

pub fn list_containers() {
    let entries = tracking::get_all_tracked_entries();
    if entries.is_empty() {
        println!("{}", "No containers tracked.".red().bold());
        return;
    }
    println!("╔════════════════════╦════════════╦══════════════╦══════════════╦══════════════════╦══════════════╦══════════════╗");
    println!(
        "║ {:<18} ║ {:<10} ║ {:<12} ║ {:<12} ║ {:<16} ║ {:<12} ║ {:<12} ║",
        "NAME".bold().truecolor(255, 165, 0),
        "PID".bold().truecolor(0, 200, 60),
        "UPTIME".bold().truecolor(150, 200, 150),
        "STATUS".bold().truecolor(150, 200, 150),
        "IMAGE".bold().truecolor(100, 150, 255),
        "PORTS".bold().truecolor(100, 150, 255),
        "ISOLATED".bold().truecolor(200, 100, 255)
    );
    println!("╠════════════════════╬════════════╬══════════════╬══════════════╬══════════════════╬══════════════╬══════════════╣");
    for entry in entries {
        let proc_path = format!("/proc/{}", entry.pid);
        let uptime_str = match tracking::get_process_uptime(entry.pid) {
            Ok(u) => {
                let seconds = u % 60;
                let minutes = (u / 60) % 60;
                let hours = (u / 3600) % 24;
                let days = u / 86400;
        
                if days > 0 {
                    format!("{}d {}h {}m {}s", days, hours, minutes, seconds)
                } else if hours > 0 {
                    format!("{}h {}m {}s", hours, minutes, seconds)
                } else if minutes > 0 {
                    format!("{}m {}s", minutes, seconds)
                } else {
                    format!("{}s", seconds)
                }
            }
            Err(_) => "N/A".to_string(),
        };
        let status = if entry.pid > 0 && Path::new(&proc_path).exists() {
            "RUNNING"
        } else if entry.pid == -2 {
            "STOPPED"
        } else {
            "EXITED"
        };
        println!(
            "║ {:<18} ║ {:<10} ║ {:<12} ║ {:<12} ║ {:<16} ║ {:<12} ║ {:<12} ║",
            entry.name,
            entry.pid,
            uptime_str,
            status,
            entry.image,
            entry.ports,
            if entry.isolated { "true" } else { "false" }
        );
    }
    println!("╚════════════════════╩════════════╩══════════════╩══════════════╩══════════════════╩══════════════╩══════════════╝");
}

pub fn stop_container(pid: i32) {
    if let Some(entry) = crate::tracking::get_all_tracked_entries().iter().find(|e| e.pid == pid) {
        kill_container(pid);
        crate::tracking::update_container_pid(
            &entry.name,
            -2,
            &entry.dir,
            &entry.command,
            &entry.image,
            &entry.ports,
            entry.isolated,
            &entry.volumes,
            &entry.env_vars,
        );
        crate::tracking::remove_container_from_tracking(pid);
        println!("Container {} has been fully removed and marked as stopped.", pid);
    } else {
        eprintln!("No container found with PID: {}", pid);
    }
}

pub fn kill_container(pid: i32) {
    let proc_path = format!("/proc/{}", pid);
    if !std::path::Path::new(&proc_path).exists() {
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
