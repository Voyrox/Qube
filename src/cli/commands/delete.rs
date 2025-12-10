use colored::*;
use std::process::exit;
use std::fs;
use crate::core::container;
use crate::config::QUBE_CONTAINERS_BASE;

pub fn delete_command(args: &[String]) {
    if args.len() < 3 {
        eprintln!("{}", "Usage: qube delete <pid|container_name>".bright_red());
        exit(1);
    }
    let identifier = args[2].clone();
    if let Ok(pid) = identifier.parse::<i32>() {
        // Delete by PID
        let tracked = crate::core::tracking::get_all_tracked_entries();
        if let Some(entry) = tracked.iter().find(|e| e.pid == pid) {
            let container_name = entry.name.clone();
            if pid > 0 {
                container::lifecycle::kill_container(pid);
            }
            crate::core::tracking::remove_container_from_tracking_by_name(&container_name);
            cleanup_container_filesystem(&container_name);
            println!("{}", format!("Deleted container {} (PID: {})", container_name, pid).green());
        } else {
            eprintln!("No container found with PID {}", pid);
            exit(1);
        }
    } else {
        // Delete by name
        let tracked = crate::core::tracking::get_all_tracked_entries();
        if let Some(entry) = tracked.iter().find(|e| e.name == identifier) {
            if entry.pid > 0 {
                container::lifecycle::kill_container(entry.pid);
            }
            crate::core::tracking::remove_container_from_tracking_by_name(&identifier);
            cleanup_container_filesystem(&identifier);
            println!("{}", format!("Deleted container {}", identifier).green());
        } else {
            // Container not in tracking, but filesystem might exist
            cleanup_container_filesystem(&identifier);
            println!("{}", format!("Cleaned up container {} (not in tracking)", identifier).green());
        }
    }
}

fn cleanup_container_filesystem(container_name: &str) {
    let container_path = format!("{}/{}", QUBE_CONTAINERS_BASE, container_name);
    if std::path::Path::new(&container_path).exists() {
        // Unmount ALL proc mounts before deleting (handles 141+ duplicate mounts)
        let proc_path = format!("{}/rootfs/proc", container_path);
        if std::path::Path::new(&proc_path).exists() {
            // Loop unmount until no proc mounts remain
            loop {
                // Check if any proc mounts still exist
                let mount_check = std::process::Command::new("mount")
                    .output()
                    .ok();
                
                if let Some(output) = mount_check {
                    let mount_output = String::from_utf8_lossy(&output.stdout);
                    if !mount_output.contains(&proc_path) {
                        break; // No more mounts
                    }
                }
                
                // Unmount one instance
                let result = std::process::Command::new("umount")
                    .arg("-l")  // Lazy unmount
                    .arg(&proc_path)
                    .status();
                
                if result.is_err() {
                    break; // Can't unmount anymore
                }
            }
        }
        
        if let Err(e) = fs::remove_dir_all(&container_path) {
            eprintln!("{}", format!("Warning: Failed to remove container directory: {}", e).yellow());
        } else {
            println!("{}", format!("  ✓ Removed container directory: {}", container_path).bright_black());
        }
    }
}

