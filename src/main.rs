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
            "Usage: qube <daemon|run|list|stop|kill|eval|...> [args...]".bright_red()
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
            let cmd_flag_index = args.iter().position(|arg| arg == "-cmd");
            if cmd_flag_index.is_none() || cmd_flag_index.unwrap() == args.len() - 1 {
                eprintln!(
                    "{}",
                    "Usage: qube run -cmd \"<command>\"".bright_red()
                );
                exit(1);
            }
            let idx = cmd_flag_index.unwrap();
            let user_cmd: Vec<String> = args[idx + 1..].to_vec();

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

            crate::tracking::track_container_named(&container_id, -1, &cwd, user_cmd);

            println!(
                "\nContainer {} registered. It will be started by the daemon.",
                container_id
            );
        }
        "list" => {
            container::list_containers();
        }
        "stop" => {
            if args.len() < 3 {
                eprintln!("{}", "Usage: qube stop <pid>".bright_red());
                exit(1);
            }
            let pid: i32 = args[2].parse().expect("Invalid PID");
            container::stop_container(pid);
        }
        "kill" => {
            if args.len() < 3 {
                eprintln!("{}", "Usage: qube kill <pid>".bright_red());
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
        _ => {
            eprintln!(
                "{}",
                format!("Unknown subcommand: {}", args[1]).bright_red()
            );
            eprintln!("Available commands: daemon, run, list, stop, kill, eval");
            exit(1);
        }
    }
}