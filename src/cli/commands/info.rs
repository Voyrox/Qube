use colored::*;
use std::process::exit;

pub fn info_command(args: &[String]) {
    if args.len() < 3 {
        eprintln!("{}", "Usage: qube info <container_name|pid>".bright_red());
        exit(1);
    }
    let identifier = &args[2];
    let tracked = crate::core::tracking::get_all_tracked_entries();
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
        match crate::core::tracking::get_process_uptime(entry.pid) {
            Ok(uptime) => println!("Uptime:      {} seconds", uptime),
            Err(_) => println!("Uptime:      N/A"),
        }
        

        println!("\n{}", "Resource Usage:".green().bold());
        match crate::core::cgroup::get_memory_stats(&entry.name) {
            Ok(stats) => {
                let percentage = if stats.max_bytes > 0 {
                    (stats.current_bytes as f64 / stats.max_bytes as f64) * 100.0
                } else {
                    0.0
                };
                let pct_format = if percentage < 1.0 {
                    format!("{:.3}%", percentage)
                } else {
                    format!("{:.1}%", percentage)
                };
                println!("Memory:      {:.2} MB / {:.2} MB ({})", 
                    stats.current_mb(),
                    stats.max_mb(),
                    pct_format
                );
            }
            Err(e) => {
                println!("Memory:      Unable to read cgroup stats ({})", e);
                // Show manual check hint
                println!("             Try: cat /sys/fs/cgroup/QubeContainers/{}/memory.current", entry.name);
            }
        }
    } else {
        eprintln!("Container with identifier {} not found in tracking.", identifier);
        exit(1);
    }
}
