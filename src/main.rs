mod daemon;
mod container;
mod cgroup;
mod tracking;
mod config;

use colored::*;
use std::env;
use std::io::{self, Write};
use std::path::Path;
use std::process::{exit, Command};

use crate::config::QUBE_CONTAINERS_BASE;
use crate::container::custom::CommandValue;

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
            "Usage: qube <daemon|run|list|stop|start|delete|eval|info|snapshot> [args...]"
                .bright_red()
        );
        exit(1);
    }

    match args[1].as_str() {
        "daemon" => {
            let _debug = args.iter().any(|arg| arg == "--debug");
            println!("{}", "Starting Qubed Daemon...".green().bold());
            daemon::start_daemon(_debug);
        }
        "run" => {
            if let Some(cmd_flag_index) = args.iter().position(|arg| arg == "--cmd") {
            let mut image = None;
            let mut ports = "".to_string();
            let mut isolated = false;

            let mut i = 2;
            while i < cmd_flag_index {
                match args[i].as_str() {
                "--image" => {
                    if let Some(val) = args.get(i + 1) {
                    image = Some(val.clone());
                    i += 2;
                    continue;
                    } else {
                    eprintln!("{}", "Usage: qube run [--image <image>] [--ports <ports>] [--isolated] [--debug] --cmd \"<command>\"".bright_red());
                    exit(1);
                    }
                }
                "--ports" => {
                    if let Some(val) = args.get(i + 1) {
                    ports = val.clone();
                    i += 2;
                    continue;
                    } else {
                    eprintln!("{}", "Usage: qube run [--image <image>] [--ports <ports>] [--isolated] [--debug] --cmd \"<command>\"".bright_red());
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

            let image = match image {
                Some(img) => img,
                None => {
                eprintln!("{}", "Error: --image flag must be specified.".bright_red());
                exit(1);
                }
            };

                if let Err(e) = crate::container::validate_image(&image) {
                    eprintln!(
                        "{}",
                        format!("Error: Invalid image provided ('{}'). Reason: {}", image, e)
                            .bright_red()
                            .bold()
                    );
                    exit(1);
                }

                let user_cmd: Vec<String> = args[cmd_flag_index + 1..].to_vec();
                let cwd = env::current_dir().map(|dir| dir.to_string_lossy().to_string()).unwrap_or_else(|e| {
                    eprintln!("{}", format!("Failed to get current directory: {}", e).bright_red());
                    exit(1);
                });

                let container_id = crate::container::lifecycle::build_container(None, &cwd, &image);           

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
                    format!("\nContainer {} built. It will be started by the daemon.", container_id)
                        .green()
                        .bold()
                );
            } else if Path::new("./qube.yml").exists() {
                println!("{}", "Detected qube.yml configuration file. Building container based on YAML configuration.".green().bold());

                let config = match crate::container::custom::read_qube_yaml() {
                    Ok(cfg) => cfg,
                    Err(e) => {
                        eprintln!("Error reading qube.yml: {}", e);
                        exit(1);
                    }
                };

                let ports = config.ports.map(|ports| ports.join(",")).unwrap_or_else(|| "".to_string());
                let isolated = config.isolated.unwrap_or(false);
                let _debug = config.debug.unwrap_or(false);
                let image = if config.system.trim().is_empty() {
                    eprintln!("{}", "Error: 'system' field in qube.yml is an invalid image.".bright_red());
                    exit(1);
                } else {
                    config.system
                };

                let cwd = env::current_dir().map(|dir| dir.to_string_lossy().to_string()).unwrap_or_else(|e| {
                    eprintln!("{}", format!("Failed to get current directory: {}", e).bright_red());
                    exit(1);
                });

                if let Err(e) = crate::container::validate_image(&image) {
                    eprintln!(
                        "{}",
                        format!("Error: Invalid image provided ('{}'). Reason: {}", image, e)
                            .bright_red()
                            .bold()
                    );
                    exit(1);
                }

                let container_id = crate::container::lifecycle::build_container(None, &cwd, &image);

                let command_str = match config.cmd {
                    CommandValue::Single(s) => s,
                    CommandValue::List(l) => l.join(" && "),
                };
                let cmd_vec = vec![command_str];                                     

                crate::tracking::track_container_named(
                    &container_id,
                    -1,
                    &cwd,
                    cmd_vec,
                    &image,
                    &ports,
                    isolated,
                );

                eprintln!(
                    "{}",
                    format!("\nContainer {} built from YAML configuration. It will be started by the daemon.", container_id)
                        .green()
                        .bold()
                );
            } else {
                eprintln!("{}", "Usage: qube run [--image <image>] [--ports <ports>] [--isolated] [--debug] --cmd \"<command>\" OR a qube.yml file must be present in the current directory.".bright_red());
                exit(1);
            }
        }
        "list" => {
            container::lifecycle::list_containers();
        }
        "stop" => {
            if args.len() < 3 {
                eprintln!("{}", "Usage: qube stop <pid|container_name>".bright_red());
                exit(1);
            }
            let identifier = args[2].clone();
            if let Ok(pid) = identifier.parse::<i32>() {
                container::lifecycle::stop_container(pid);
            } else {
                let tracked = crate::tracking::get_all_tracked_entries();
                if let Some(entry) = tracked.iter().find(|e| e.name == identifier) {
                    container::lifecycle::stop_container(entry.pid);
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
                if entry.pid > 0 && Path::new(&format!("/proc/{}", entry.pid)).exists() {
                    println!("Container {} is already running (PID: {}).", entry.name, entry.pid);
                } else if entry.pid == -1 {
                    println!("Container {} is already scheduled to start.", entry.name);
                } else {
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
            container::lifecycle::kill_container(pid);
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
            eprintln!("{}", format!("Unknown subcommand: {}", args[1]).bright_red());
            eprintln!("Available commands: daemon, run, list, stop, start, delete, eval, info, snapshot");
            exit(1);
        }
    }
}
