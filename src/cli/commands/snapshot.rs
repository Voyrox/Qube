use colored::*;
use std::process::{exit, Command};
use crate::config::QUBE_CONTAINERS_BASE;

pub fn snapshot_command(args: &[String]) {
    if args.len() < 3 {
        eprintln!("{}", "Usage: qube snapshot <container_name|pid>".bright_red());
        exit(1);
    }
    let identifier = &args[2];
    let tracked = crate::core::tracking::get_all_tracked_entries();
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
