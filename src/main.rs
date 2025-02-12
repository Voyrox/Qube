mod daemon;
mod container;
mod cgroup;
mod tracking;

use colored::*;
use std::env;
use std::process::exit;
use rand::{distributions::Alphanumeric, Rng};
use std::process::Command;
use std::io::{self, Write};

const QUBE_CONTAINERS_BASE: &str = "/var/tmp/Qube_containers";

fn main() {
    if nix::unistd::geteuid().as_raw() != 0 {
        eprintln!(
            "{}",
            "Error: This program must be run as root (use sudo)."
                .bright_red()
                .bold()
        );
        exit(1);
    }

    let args: Vec<String> = env::args().collect();
    if args.len() < 2 {
        eprintln!(
            "{}",
            "Usage: qube <daemon|run|list|stop|start|delete|eval|info|snapshot> [args...]".bright_red()
        );
        exit(1);
    }

    match args[1].as_str() {
        "daemon" => {
            let debug = args.iter().any(|arg| arg == "--debug");
            println!("{}", "Starting Qubed Daemon...".green().bold());
            daemon::start_daemon(debug);
        }
        "run" => {
            let cmd_flag_index = args.iter().position(|arg| arg == "--cmd");
            if cmd_flag_index.is_none() || cmd_flag_index.unwrap() == args.len() - 1 {
                eprintln!("{}", "Usage: Qube run [--image <image>] [--ports <ports>] [--isolated] --cmd \"<command>\"".bright_red());
                exit(1);
            }
            let cmd_index = cmd_flag_index.unwrap();

            let mut image = "ubuntu".to_string();
            let mut ports = "".to_string();
            let mut isolated = false;

            let mut i = 2;
            while i < cmd_index {
                match args[i].as_str() {
                    "--image" => {
                        if i + 1 < cmd_index {
                            image = args[i + 1].clone();
                            i += 2;
                            continue;
                        } else {
                            eprintln!("{}", "Usage: qube run [--image <image>] [--ports <ports>] [--isolated] --cmd \"<command>\"".bright_red());
                            exit(1);
                        }
                    }
                    "--ports" => {
                        if i + 1 < cmd_index {
                            ports = args[i + 1].clone();
                            i += 2;
                            continue;
                        } else {
                            eprintln!("{}", "Usage: qube run [--image <image>] [--ports <ports>] [--isolated] --cmd \"<command>\"".bright_red());
                            exit(1);
                        }
                    }
                    "--isolated" => {
                        isolated = true;
                        i += 1;
                        continue;
                    }
                    _ => {
                        i += 1;
                    }
                }
            }

            let user_cmd: Vec<String> = args[cmd_index + 1..].to_vec();

            let cwd = match env::current_dir() {
                Ok(dir) => dir.to_string_lossy().to_string(),
                Err(e) => {
                    eprintln!(
                        "{}",
                        format!("Failed to get current directory: {}", e).bright_red()
                    );
                    exit(1);
                }
            };

            let rand_str: String = rand::thread_rng()
                .sample_iter(&Alphanumeric)
                .take(6)
                .map(char::from)
                .collect();
            let container_id = format!("Qube-{}", rand_str);

            crate::tracking::track_container_named(
                &container_id,
                -1,
                &cwd,
                user_cmd.clone(),
                &image,
                &ports,
                isolated,
            );

            eprintln!(
                "{}",
                format!("\nContainer {} registered. It will be started by the daemon.", container_id)
                    .green()
                    .bold()
            );
        }
        "list" => {
            container::list_containers();
        }
        "stop" => {
            if args.len() < 3 {
                eprintln!("{}", "Usage: qube stop <pid|container_name>".bright_red());
                exit(1);
            }
            let identifier = args[2].clone();
            if let Ok(pid) = identifier.parse::<i32>() {
                container::stop_container(pid);
            } else {
                let tracked = crate::tracking::get_all_tracked_entries();
                if let Some(entry) = tracked.iter().find(|e| e.name == identifier) {
                    container::stop_container(entry.pid);
                } else {
                    eprintln!("No container found with identifier {}", identifier);
                    exit(1);
                }
            }
        }
        "start" => {
            if args.len() < 3 {
                eprintln!("{}", "Usage: qube start <container_name|pid>".bright_red());
                exit(1);
            }
            let identifier = &args[2];
            let tracked = crate::tracking::get_all_tracked_entries();
            let entry_opt = tracked.iter().find(|e| {
                e.name == *identifier || e.pid.to_string() == *identifier
            });
            if let Some(entry) = entry_opt {
                // If the container is running, do nothing
                if entry.pid > 0 && std::path::Path::new(&format!("/proc/{}", entry.pid)).exists() {
                    println!("Container {} is already running (PID: {}).", entry.name, entry.pid);
                } else if entry.pid == -1 {
                    println!("Container {} is already scheduled to start.", entry.name);
                } else {
                    // Set pid to -1 to indicate that the daemon should (re)start it.
                    crate::tracking::update_container_pid(
                        &entry.name,
                        -1,
                        &entry.dir,
                        &entry.command,
                        &entry.image,
                        &entry.ports,
                        entry.isolated,
                    );
                    println!("Container {} marked to start. The daemon will start it shortly.", entry.name);
                }
            } else {
                eprintln!("Container with identifier {} not found in tracking.", identifier);
                exit(1);
            }
        }
        "delete" => {
            if args.len() < 3 {
                eprintln!("{}", "Usage: qube delete <pid>".bright_red());
                exit(1);
            }
            let pid: i32 = args[2].parse().expect("Invalid PID");
            container::kill_container(pid);
        }
        "eval" => {
            if args.len() < 3 {
                eprintln!("{}", "Usage: qube eval <container_name|pid> [command]".bright_red());
                exit(1);
            }
            let container_identifier = &args[2];
            let command_to_run = if args.len() >= 4 {
                args[3..].join(" ")
            } else {
                "/bin/bash".to_string()
            };

            let tracked = crate::tracking::get_all_tracked_entries();
            let entry_opt = tracked.iter().find(|e| {
                e.name == *container_identifier || e.pid.to_string() == *container_identifier
            });
            if let Some(entry) = entry_opt {
                println!(
                    "{}",
                    format!(
                        "WARNING: You are about to attach to container {} (PID: {}). Running commands as root inside the container is dangerous. Make sure you understand the security implications!",
                        entry.name, entry.pid
                    )
                    .bright_yellow()
                    .bold()
                );
                print!("Proceed? (y/n): ");
                io::stdout().flush().unwrap();
                let mut input = String::new();
                io::stdin().read_line(&mut input).expect("Failed to read input");
                if input.trim().to_lowercase() != "y" {
                    println!("Aborted.");
                    exit(0);
                }

                let nsenter_cmd = format!("nsenter -t {} -a {}", entry.pid, command_to_run);
                println!("Executing: {}", nsenter_cmd);
                let status = Command::new("sh")
                    .arg("-c")
                    .arg(nsenter_cmd)
                    .status()
                    .expect("Failed to execute nsenter command");
                exit(status.code().unwrap_or(1));
            } else {
                eprintln!("Container with identifier {} not found in tracking.", container_identifier);
                exit(1);
            }
        }
        "info" => {
            if args.len() < 3 {
                eprintln!("{}", "Usage: qube info <container_name|pid>".bright_red());
                exit(1);
            }
            let identifier = &args[2];
            let tracked = crate::tracking::get_all_tracked_entries();
            let entry_opt = tracked.iter().find(|e| {
                e.name == *identifier || e.pid.to_string() == *identifier
            });
            if let Some(entry) = entry_opt {
                println!("{}", "Container Information:".green().bold());
                println!("Name:        {}", entry.name);
                println!("PID:         {}", entry.pid);
                println!("Working Dir: {}", entry.dir);
                println!("Command:     {}", entry.command.join(" "));
                println!("Timestamp:   {}", entry.timestamp);
                println!("Image:       {}", entry.image);
                println!("Ports:       {}", entry.ports);
                println!("Isolated:    {}", if entry.isolated { "ENABLED" } else { "DISABLED" });
                match crate::tracking::get_process_uptime(entry.pid) {
                    Ok(uptime) => println!("Uptime:      {} seconds", uptime),
                    Err(_) => println!("Uptime:      N/A"),
                }
            } else {
                eprintln!("Container with identifier {} not found in tracking.", identifier);
                exit(1);
            }
        }
        "snapshot" => {
            if args.len() < 3 {
                eprintln!("{}", "Usage: qube snapshot <container_name|pid>".bright_red());
                exit(1);
            }
            let identifier = &args[2];
            let tracked = crate::tracking::get_all_tracked_entries();
            let entry_opt = tracked.iter().find(|e| {
                e.name == *identifier || e.pid.to_string() == *identifier
            });
            if let Some(entry) = entry_opt {
                let rootfs_path = format!("{}/{}/rootfs", QUBE_CONTAINERS_BASE, entry.name);
                let snapshot_path = format!("{}/snapshot-{}.tar.gz", entry.dir, entry.name);
                println!("Creating snapshot of {} into {}", rootfs_path, snapshot_path);
                let status = Command::new("tar")
                    .args(&["-czf", &snapshot_path, "-C", &rootfs_path, "."])
                    .status()
                    .expect("Failed to execute tar command");
                if status.success() {
                    println!("Snapshot created successfully at {}", snapshot_path);
                } else {
                    eprintln!("Snapshot creation failed.");
                }
            } else {
                eprintln!("Container with identifier {} not found in tracking.", identifier);
                exit(1);
            }
        }
        _ => {
            eprintln!(
                "{}",
                format!("Unknown subcommand: {}", args[1]).bright_red()
            );
            eprintln!("Available commands: daemon, run, list, stop, start, delete, eval, info, snapshot");
            exit(1);
        }
    }
}
