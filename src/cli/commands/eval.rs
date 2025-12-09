use colored::*;
use std::io::{self, Write};
use std::process::{exit, Command};

pub fn eval_command(args: &[String]) {
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

    let tracked = crate::core::tracking::get_all_tracked_entries();
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
