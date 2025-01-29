use std::fs::{self, File};
use std::io::{Read, Write, Error, ErrorKind};

pub const TRACKING_DIR: &str = "/var/lib/Qube";
pub const CONTAINER_LIST_FILE: &str = "/var/lib/Qube/containers.txt";

pub fn track_container(pid: i32) {
    fs::create_dir_all(TRACKING_DIR).expect("Failed to create tracking directory");

    let mut file = fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(CONTAINER_LIST_FILE)
        .expect("Failed to open container tracking file");

    writeln!(file, "{}", pid).expect("Failed to write PID to tracking file");
}

pub fn remove_container_from_tracking(pid: i32) {
    if let Ok(contents) = fs::read_to_string(CONTAINER_LIST_FILE) {
        let new_contents: Vec<String> = contents
            .lines()
            .filter(|&line_pid| line_pid != pid.to_string())
            .map(|s| s.to_string())
            .collect();
        fs::write(CONTAINER_LIST_FILE, new_contents.join("\n"))
            .expect("Failed to update container list");
    }
}

pub fn get_process_uptime(pid: i32) -> Result<u64, std::io::Error> {
    let stat_path = format!("/proc/{}/stat", pid);
    let mut contents = String::new();
    File::open(&stat_path)?.read_to_string(&mut contents)?;

    let fields: Vec<&str> = contents.split_whitespace().collect();
    if fields.len() <= 21 {
        return Err(Error::new(ErrorKind::Other, "Failed to parse process stat fields"));
    }

    let start_time_in_jiffies: f64 = fields[21].parse().unwrap_or(0.0);

    let mut uptime_str = String::new();
    File::open("/proc/uptime")?.read_to_string(&mut uptime_str)?;
    let system_uptime: f64 = uptime_str
        .split_whitespace()
        .next()
        .unwrap_or("0.0")
        .parse()
        .unwrap_or(0.0);

    let hertz: f64 = 100.0; // or dynamically: get_clock_ticks_per_sec()
    let process_start_sec: f64 = start_time_in_jiffies / hertz;

    let process_uptime = system_uptime - process_start_sec;
    
    Ok(process_uptime.max(0.0) as u64)
}