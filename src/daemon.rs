use crate::tracking::{self};
use colored::Colorize;
use nix::sys::signal::{self, Signal};
use std::sync::atomic::{AtomicBool, Ordering};
use std::thread;
use std::time::Duration;
use std::fs;

static RUNNING: AtomicBool = AtomicBool::new(true);

extern "C" fn handle_signal(_: i32) {
    RUNNING.store(false, Ordering::SeqCst);
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
            let pid = entry.pid;
            if !is_process_alive(pid) {
                eprintln!(
                    "{}",
                    format!("Container with PID {} seems to have exited. (ID={})", pid, entry.name)
                        .yellow()
                        .bold()
                );

                if !entry.command.is_empty() {
                    eprintln!(
                        "{}",
                        format!("Attempting to restart container: {}...", entry.name).blue().bold()
                    );

                    tracking::remove_container_from_tracking(pid);

                    crate::container::run_container(
                        Some(&entry.name),
                        &entry.dir,
                        &entry.command,
                        debug,
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

fn is_process_alive(pid: i32) -> bool {
    let proc_path = format!("/proc/{}/status", pid);
    
    if !std::path::Path::new(&proc_path).exists() {
        return false;
    }

    if let Ok(status) = fs::read_to_string(proc_path) {
        return status.contains("State:\tR");
    }
    
    false
}
