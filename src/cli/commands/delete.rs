use colored::*;
use std::process::exit;
use crate::core::container;

pub fn delete_command(args: &[String]) {
    if args.len() < 3 {
        eprintln!("{}", "Usage: qube delete <pid|container_name>".bright_red());
        exit(1);
    }
    let identifier = args[2].clone();
    if let Ok(pid) = identifier.parse::<i32>() {
        container::lifecycle::kill_container(pid);
    } else {
        let tracked = crate::core::tracking::get_all_tracked_entries();
        if let Some(entry) = tracked.iter().find(|e| e.name == identifier) {
            container::lifecycle::kill_container(entry.pid);
        } else {
            eprintln!("No container found with identifier {}", identifier);
            exit(1);
        }
    }
}
