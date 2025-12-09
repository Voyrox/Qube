use colored::*;
use std::path::Path;
use std::process::exit;

pub fn start_command(args: &[String]) {
    if args.len() < 3 {
        eprintln!("{}", "Usage: qube start <container_name|pid>".bright_red());
        exit(1);
    }
    let identifier = &args[2];
    let tracked = crate::core::tracking::get_all_tracked_entries();
    let entry_opt = tracked.iter().find(|e| {
        e.name == *identifier || e.pid.to_string() == *identifier
    });
    if let Some(entry) = entry_opt {
        if entry.pid > 0 && Path::new(&format!("/proc/{}", entry.pid)).exists() {
            println!("Container {} is already running (PID: {}).", entry.name, entry.pid);
        } else if entry.pid == -1 {
            println!("Container {} is already scheduled to start.", entry.name);
        } else {
            crate::core::tracking::update_container_pid(
                &entry.name,
                -1,
                &entry.dir,
                &entry.command,
                &entry.image,
                &entry.ports,
                entry.isolated,
                &entry.volumes,
                &entry.env_vars,
            );
            println!("Container {} marked to start. The daemon will start it shortly.", entry.name);
        }
    } else {
        eprintln!("Container with identifier {} not found in tracking.", identifier);
        exit(1);
    }
}
