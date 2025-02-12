use crate::tracking;
use colored::Colorize;
use nix::sys::signal::{self, Signal};
use std::sync::atomic::{AtomicBool, Ordering};
use std::thread;
use std::time::{Duration, SystemTime, UNIX_EPOCH};
use std::fs;

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

pub fn start_daemon(debug: bool) -> ! {
    unsafe {
        let _ = signal::signal(Signal::SIGTERM, signal::SigHandler::Handler(handle_signal));
        let _ = signal::signal(Signal::SIGINT, signal::SigHandler::Handler(handle_signal));
    }

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

                    crate::container::run_container(
                        Some(&entry.name),
                        &entry.dir,
                        &entry.command,
                        debug,
                        &entry.image,
                        &entry.ports,
                        entry.isolated,
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
