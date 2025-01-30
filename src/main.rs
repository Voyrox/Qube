mod daemon;
mod container;
mod cgroup;
mod tracking;

use colored::*;
use std::env;
use std::process::exit;

fn main() {
    if nix::unistd::geteuid().as_raw() != 0 {
        eprintln!("{}", "Error: This program must be run as root (use sudo).".bright_red().bold());
        exit(1);
    }

    let args: Vec<String> = env::args().collect();
    if args.len() < 2 {
        eprintln!(
            "{}",
            "Usage: qube <daemon|run|list|stop|kill> [args...]".bright_red()
        );
        exit(1);
    }

    match args[1].as_str() {
        "daemon" => {
            println!("{}", "Starting Qubed Daemon...".green().bold());
            daemon::start_daemon();
        }
        "run" => {
            let container_cmd: Vec<String> = args[2..].to_vec();
            if container_cmd.is_empty() {
                eprintln!("{}", "Please specify a command to run inside the container, e.g. /bin/bash".bright_red());
                exit(1);
            }
            container::run_container(&container_cmd);
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
        _ => {
            eprintln!("{}", format!("Unknown subcommand: {}", args[1]).bright_red());
            exit(1);
        }
    }
}
