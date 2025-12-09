use crate::core::container::{fs, image};
use crate::core::tracking;
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
            crate::core::tracking::remove_container_from_tracking_by_name(&container_id);
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
        println!("\n  {}", "No containers running".bright_black().italic());
        println!("  {} {}\n", "→".bright_blue(), "Use 'qube run' to start a container".bright_black());
        return;
    }
    
    println!("\n{}", "  CONTAINERS".bold().bright_white());
    println!("{}", "  ───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────".bright_black());
    
    for entry in &entries {
        let proc_path = format!("/proc/{}", entry.pid);
        
        let (status_icon, status_color) = if entry.pid > 0 && Path::new(&proc_path).exists() {
            ("●", colored::Color::Green)
        } else if entry.pid == -2 {
            ("■", colored::Color::Red)
        } else {
            ("▲", colored::Color::Yellow)
        };
        
        let status_text = if entry.pid > 0 && Path::new(&proc_path).exists() {
            "running".green()
        } else if entry.pid == -2 {
            "stopped".red()
        } else {
            "exited".yellow()
        };
        
        // Memory
        let mem = match crate::core::cgroup::get_memory_stats(&entry.name) {
            Ok(s) => {
                let mb = s.current_mb();
                if mb < 100.0 {
                    format!("{:.1}M", mb).green()
                } else if mb < 1024.0 {
                    format!("{:.0}M", mb).yellow()
                } else {
                    format!("{:.1}G", mb / 1024.0).red()
                }
            }
            Err(_) => "─".bright_black(),
        };
        
        // Uptime
        let uptime = match tracking::get_process_uptime(entry.pid) {
            Ok(u) => {
                let d = u / 86400;
                let h = (u / 3600) % 24;
                let m = (u / 60) % 60;
                if d > 0 {
                    format!("{}d{}h", d, h).cyan()
                } else if h > 0 {
                    format!("{}h{}m", h, m).cyan()
                } else {
                    format!("{}m", m).cyan()
                }
            }
            Err(_) => "─".bright_black(),
        };
        
        let cmd = entry.command.join(" ");
        let truncated = if cmd.len() > 70 { format!("{}...", &cmd[..67]) } else { cmd };
        
        // Ports
        let ports_str = if entry.ports.is_empty() {
            "none".bright_black()
        } else {
            entry.ports.bright_magenta()
        };
        
        // Isolation
        let isolation_str = if entry.isolated {
            "isolated".bright_yellow()
        } else {
            "shared".bright_black()
        };
        
        println!(
            "  {} {}  {} │ {}  │  pid: {}  │  mem: {}  │  up: {}  │  ports: {}  │  net: {}",
            status_icon.color(status_color).bold(),
            entry.name.bright_white().bold(),
            format!("[{}]", status_text),
            entry.image.bright_blue(),
            if entry.pid > 0 { entry.pid.to_string().cyan() } else { "─".bright_black() },
            mem,
            uptime,
            ports_str,
            isolation_str
        );
        println!("     {}", truncated.bright_black().italic());
        println!();
    }
    
    println!("{}", "  ───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────".bright_black());
    println!("  {} {}\n", entries.len().to_string().cyan().bold(), "containers".bright_black());
}

pub fn stop_container(pid: i32) {
    if let Some(entry) = crate::core::tracking::get_all_tracked_entries().iter().find(|e| e.pid == pid) {
        kill_container(pid);
        crate::core::tracking::update_container_pid(
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
        crate::core::tracking::remove_container_from_tracking(pid);
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
    
    // Get container name before killing for cgroup cleanup
    let container_name = tracking::get_all_tracked_entries()
        .iter()
        .find(|e| e.pid == pid)
        .map(|e| e.name.clone());
    
    if kill(Pid::from_raw(pid), Signal::SIGKILL).is_ok() {
        println!("Killed container with PID: {}", pid);
        tracking::remove_container_from_tracking(pid);
        
        // Clean up cgroup
        if let Some(name) = container_name {
            if let Err(e) = crate::core::cgroup::cleanup_cgroup(&name) {
                eprintln!("Warning: Failed to cleanup cgroup for {}: {}", name, e);
            }
        }
    } else {
        println!("Failed to kill container with PID: {}", pid);
    }
}
