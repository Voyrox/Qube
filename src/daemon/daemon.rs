use crate::{api, core::tracking, config};
use colored::Colorize;
use nix::sys::signal::{self, Signal};
use std::sync::atomic::{AtomicBool, Ordering};
use std::thread;
use std::time::{Duration, SystemTime, UNIX_EPOCH};
use std::fs;
use std::path::PathBuf;

static RUNNING: AtomicBool = AtomicBool::new(true);

extern "C" fn handle_signal(_: i32) {
    RUNNING.store(false, Ordering::SeqCst);
}

fn current_timestamp() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs()
}

fn cleanup_orphaned_containers() {
    let tracked_containers = tracking::get_all_tracked_entries();
    let tracked_names: std::collections::HashSet<String> = tracked_containers
        .iter()
        .map(|entry| entry.name.clone())
        .collect();

    let containers_path = PathBuf::from(config::QUBE_CONTAINERS_BASE);
    if !containers_path.exists() {
        return;
    }

    if let Ok(entries) = fs::read_dir(&containers_path) {
        for entry in entries.flatten() {
            let path = entry.path();
            if path.is_dir() {
                if let Some(dir_name) = path.file_name().and_then(|n| n.to_str()) {
                    if dir_name == "images" {
                        continue;
                    }

                    if !tracked_names.contains(dir_name) {
                        eprintln!(
                            "{}",
                            format!("Found orphaned container directory: {}. Removing...", dir_name).yellow()
                        );

                        let proc_path = format!("{}/rootfs/proc", path.display());
                        loop {
                            let mount_check = std::process::Command::new("mount")
                                .output()
                                .ok();
                            
                            if let Some(output) = mount_check {
                                let mount_output = String::from_utf8_lossy(&output.stdout);
                                if !mount_output.contains(&proc_path) {
                                    break;
                                }
                            } else {
                                break;
                            }

                            let _ = std::process::Command::new("umount")
                                .arg("-l")
                                .arg(&proc_path)
                                .status();
                        }

                        if let Err(e) = fs::remove_dir_all(&path) {
                            eprintln!("{}", format!("Failed to remove orphaned container {}: {}", dir_name, e).red());
                        } else {
                            eprintln!("{}", format!("✓ Removed orphaned container directory: {}", dir_name).green());
                        }
                    }
                }
            }
        }
    }
}

pub fn start_daemon(debug: bool) -> ! {
    unsafe {
        let _ = signal::signal(Signal::SIGTERM, signal::SigHandler::Handler(handle_signal));
        let _ = signal::signal(Signal::SIGINT, signal::SigHandler::Handler(handle_signal));
    }

    cleanup_orphaned_containers();

    if let Err(e) = crate::core::cgroup::init_cgroup_root() {
        eprintln!("{}", format!("Warning: Failed to initialize cgroup root: {}", e).yellow());
        eprintln!("{}", "Containers will run without resource limits.".yellow());
    } else if debug {
        println!("{}", "Cgroup root initialized successfully.".green());
    }

    thread::spawn(|| {
        api::start_server();
    });
    
    println!("{}", "Qubed Daemon started successfully.".green().bold());
    println!("Press Ctrl+C or send SIGTERM to stop.");

    while RUNNING.load(Ordering::SeqCst) {
        let all_tracked = tracking::get_all_tracked_entries();

        for entry in &all_tracked {
            if entry.pid == -2 {
                continue;
            }
            if entry.pid == -1 && current_timestamp().saturating_sub(entry.timestamp) < 5 {
                continue;
            }
 
            if entry.pid >= -1 && !is_process_alive(entry.pid) {
                eprintln!(
                    "{}",
                    format!("Container with PID {} seems to have exited. (ID={})", entry.pid, entry.name)
                        .yellow()
                        .bold()
                );

                tracking::remove_container_from_tracking_by_name(&entry.name);

                if !entry.command.is_empty() {
                    eprintln!(
                        "{}",
                        format!("Attempting to restart container: {}...", entry.name).blue().bold()
                    );

                    crate::core::container::run_container(
                        Some(&entry.name),
                        &entry.dir,
                        &entry.command,
                        debug,
                        &entry.image,
                        &entry.ports,
                        entry.isolated,
                        &[] as &[(String, String)],
                        &entry.env_vars,
                    );
                } else {
                    eprintln!(
                        "{}",
                        format!(
                            "No command recorded for container {}, cannot automatically restart",
                            entry.name
                        )
                        .red()
                        .bold()
                    );
                }
            }
        }

        thread::sleep(Duration::from_secs(5));
    }

    println!("{}", "Qubed Daemon shutting down...".green().bold());
    std::process::exit(0);
}

pub fn is_process_alive(pid: i32) -> bool {
    let proc_path = format!("/proc/{}/status", pid);
    
    if !std::path::Path::new(&proc_path).exists() {
        return false;
    }

    if let Ok(status) = fs::read_to_string(proc_path) {
        if let Some(line) = status.lines().find(|l| l.starts_with("State:")) {
            if line.contains("Z") {
                return false;
            } else {
                return true;
            }
        }
    }
    false
}
