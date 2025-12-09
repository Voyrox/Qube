mod daemon;
mod core;
mod config;
mod api;
mod cli;

use colored::*;
use std::env;
use std::process::exit;

fn main() {
    if cfg!(target_os = "windows") {
        eprintln!("{}", "Warning: Native container isolation is not fully supported on Windows.".bright_red());
        exit(1);
    }

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
            "Usage: qube <daemon|run|list|stop|start|delete|eval|info|snapshot|docker> [args...]"
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
            cli::run_command(&args);
        }
        "list" => {
            cli::list_command();
        }
        "stop" => {
            cli::stop_command(&args);
        }
        "start" => {
            cli::start_command(&args);
        }
        "delete" => {
            cli::delete_command(&args);
        }
        "eval" => {
            cli::eval_command(&args);
        }
        "info" => {
            cli::info_command(&args);
        }
        "snapshot" => {
            cli::snapshot_command(&args);
        }
        "docker" => {
            cli::docker_command(&args);
        }
        _ => {
            eprintln!("{}", format!("Unknown subcommand: {}", args[1]).bright_red());
            eprintln!("Available commands: daemon, run, list, stop, start, delete, eval, info, snapshot");
            exit(1);
        }
    }
}
