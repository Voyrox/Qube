use colored::*;
use std::process::exit;
use crate::core::container;

pub fn stop_command(args: &[String]) {
    if args.len() < 3 {
        eprintln!("{}", "Usage: qube stop <pid|container_name>".bright_red());
        exit(1);
    }
    let identifier = args[2].clone();
    if let Ok(pid) = identifier.parse::<i32>() {
        container::lifecycle::stop_container(pid);
    } else {
        let tracked = crate::core::tracking::get_all_tracked_entries();
        if let Some(entry) = tracked.iter().find(|e| e.name == identifier) {
            container::lifecycle::stop_container(entry.pid);
        } else {
            eprintln!("No container found with identifier {}", identifier);
            exit(1);
        }
    }
}
