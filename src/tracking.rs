use std::fs;
use std::io::{Read, Write, ErrorKind};
use std::fs::File;
use std::path::Path;

pub const TRACKING_DIR: &str = "/var/lib/Qube";
pub const CONTAINER_LIST_FILE: &str = "/var/lib/Qube/containers.txt";

#[derive(Debug)]
pub struct ContainerEntry {
    pub name: String,
    pub pid: i32,
    pub command: Vec<String>,
}

pub fn track_container_named(name: &str, pid: i32, cmd: Vec<String>) {
    fs::create_dir_all(TRACKING_DIR).ok();
    let command_str = cmd.join("\t");
    let line = format!("{} {} {}", name, pid, command_str);

    let mut file = fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(CONTAINER_LIST_FILE)
        .expect("Failed to open container tracking file");
    let _ = writeln!(file, "{}", line);
}

pub fn remove_container_from_tracking(pid: i32) {
    if let Ok(contents) = fs::read_to_string(CONTAINER_LIST_FILE) {
        let mut new_contents = Vec::new();
        for line in contents.lines() {
            let parts: Vec<&str> = line.trim().split_whitespace().collect();
            if parts.len() < 2 { 
                continue; 
            }
            if let Ok(line_pid) = parts[1].parse::<i32>() {
                if line_pid == pid {
                    continue;
                }
            }
            new_contents.push(line.to_string());
        }
        fs::write(CONTAINER_LIST_FILE, new_contents.join("\n"))
            .expect("Failed to update container list");
    }
}

pub fn get_running_containers() -> Vec<i32> {
    let mut running_pids = Vec::new();

    if let Ok(contents) = fs::read_to_string(CONTAINER_LIST_FILE) {
        for line in contents.lines() {
            let parts: Vec<&str> = line.trim().split_whitespace().collect();
            if parts.len() >= 2 {
                if let Ok(pid) = parts[1].parse::<i32>() {
                    let proc_path = format!("/proc/{}", pid);
                    if Path::new(&proc_path).exists() {
                        running_pids.push(pid);
                    }
                }
            }
        }
    }
    running_pids
}

pub fn get_all_tracked_entries() -> Vec<ContainerEntry> {
    let mut entries = Vec::new();

    if let Ok(contents) = fs::read_to_string(CONTAINER_LIST_FILE) {
        for line in contents.lines() {
            let parts: Vec<&str> = line.trim().split_whitespace().collect();
            if parts.len() < 2 {
                continue;
            }
            let name = parts[0].to_string();
            let pid = parts[1].parse::<i32>().unwrap_or(-1);

            let remainder = &line[name.len() + 1 + parts[1].len()..].trim();
            let cmd_parts: Vec<String> = remainder.split('\t').map(|s| s.to_string()).collect();

            entries.push(ContainerEntry {
                name,
                pid,
                command: cmd_parts,
            });
        }
    }

    entries
}

pub fn get_process_uptime(pid: i32) -> Result<u64, std::io::Error> {
    let path = format!("/proc/{}/stat", pid);
    let mut buf = String::new();
    File::open(&path)?.read_to_string(&mut buf)?;
    let fields: Vec<&str> = buf.split_whitespace().collect();
    if fields.len() <= 21 {
        return Err(std::io::Error::new(ErrorKind::Other, "Failed to parse stat"));
    }
    let start_time: f64 = fields[21].parse().unwrap_or(0.0);
    let mut uptime_str = String::new();
    File::open("/proc/uptime")?.read_to_string(&mut uptime_str)?;
    let sys_uptime: f64 = uptime_str.split_whitespace().next().unwrap_or("0").parse().unwrap_or(0.0);
    let hertz = 100.0;
    let start_sec = start_time / hertz;
    let proc_uptime = sys_uptime - start_sec;
    Ok(proc_uptime.max(0.0) as u64)
}
