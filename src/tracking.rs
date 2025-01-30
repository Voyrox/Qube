use std::fs;
use std::io::{Read, Write, ErrorKind};
use std::fs::File;

pub const TRACKING_DIR: &str = "/var/lib/Qube";
pub const CONTAINER_LIST_FILE: &str = "/var/lib/Qube/containers.txt";

pub fn track_container_named(name: &str, pid: i32) {
    fs::create_dir_all(TRACKING_DIR).ok();
    let mut file = fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(CONTAINER_LIST_FILE)
        .expect("Failed to open container tracking file");
    let _ = writeln!(file, "{} {}", name, pid);
}

pub fn remove_container_from_tracking(pid: i32) {
    if let Ok(contents) = fs::read_to_string(CONTAINER_LIST_FILE) {
        let mut new_contents = Vec::new();
        for line in contents.lines() {
            let parts: Vec<&str> = line.trim().split_whitespace().collect();
            if parts.len() < 2 { continue; }
            if parts[1] != pid.to_string() {
                new_contents.push(line.to_string());
            }
        }
        fs::write(CONTAINER_LIST_FILE, new_contents.join("\n"))
            .expect("Failed to update container list");
    }
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