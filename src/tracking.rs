use std::fs;
use std::io::{Read, Write, Seek, SeekFrom};
use std::fs::File;
use std::time::{SystemTime, UNIX_EPOCH};

pub const TRACKING_DIR: &str = "/var/lib/Qube";
pub const CONTAINER_LIST_FILE: &str = "/var/lib/Qube/containers.txt";

#[derive(Debug)]
pub struct ContainerEntry {
    pub name: String,
    pub pid: i32,
    pub dir: String,
    pub command: Vec<String>,
    pub timestamp: u64,
}

fn current_timestamp() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs()
}

/// Format: name|pid|dir|command|timestamp
pub fn track_container_named(n: &str, p: i32, d: &str, c: Vec<String>) {
    fs::create_dir_all(TRACKING_DIR).ok();
    let timestamp = current_timestamp();
    let s = c.join("\t");
    let line = format!("{}|{}|{}|{}|{}", n, p, d, s, timestamp);

    let mut f = fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(CONTAINER_LIST_FILE)
        .expect("Failed to open container tracking file");

    if let Ok(metadata) = fs::metadata(CONTAINER_LIST_FILE) {
        if metadata.len() > 0 {
            let mut file = File::open(CONTAINER_LIST_FILE).unwrap();
            file.seek(SeekFrom::End(-1)).unwrap();
            let mut buf = [0u8; 1];
            file.read_exact(&mut buf).unwrap();
            if buf[0] != b'\n' {
                f.write_all(b"\n").unwrap();
            }
        }
    }

    writeln!(f, "{}", line).unwrap();
}

pub fn update_container_pid(name: &str, new_pid: i32, new_dir: &str, new_cmd: &[String]) {
    let mut found = false;
    let new_line = format!("{}|{}|{}|{}|{}", name, new_pid, new_dir, new_cmd.join("\t"), current_timestamp());
    if let Ok(c) = fs::read_to_string(CONTAINER_LIST_FILE) {
        let mut new_lines = Vec::new();
        for l in c.lines() {
            let parts: Vec<&str> = l.splitn(5, '|').collect();
            if parts.len() < 4 {
                new_lines.push(l.to_string());
                continue;
            }
            if parts[0] == name {
                new_lines.push(new_line.clone());
                found = true;
            } else {
                new_lines.push(l.to_string());
            }
        }
        let joined = new_lines.join("\n");
        fs::write(CONTAINER_LIST_FILE, joined).unwrap();
    }
    if !found {
        track_container_named(name, new_pid, new_dir, new_cmd.to_vec());
    }
}

pub fn remove_container_from_tracking(pid: i32) {
    if let Ok(c) = fs::read_to_string(CONTAINER_LIST_FILE) {
        let mut nc = Vec::new();
        for l in c.lines() {
            let parts: Vec<&str> = l.splitn(5, '|').collect();
            if parts.len() < 4 {
                continue;
            }
            if let Ok(x) = parts[1].parse::<i32>() {
                if x == pid {
                    continue;
                }
            }
            nc.push(l.to_string());
        }
        let joined = nc.join("\n");
        fs::write(CONTAINER_LIST_FILE, joined).unwrap();
    }
}

pub fn remove_container_from_tracking_by_name(name: &str) {
    if let Ok(c) = fs::read_to_string(CONTAINER_LIST_FILE) {
        let new_lines: Vec<String> = c
            .lines()
            .filter(|l| {
                let parts: Vec<&str> = l.splitn(5, '|').collect();
                if parts.len() < 4 {
                    return true;
                }
                parts[0] != name
            })
            .map(String::from)
            .collect();
        let joined = new_lines.join("\n");
        fs::write(CONTAINER_LIST_FILE, joined).unwrap();
    }
}

pub fn get_all_tracked_entries() -> Vec<ContainerEntry> {
    let mut v = Vec::new();
    if let Ok(c) = fs::read_to_string(CONTAINER_LIST_FILE) {
        for l in c.lines() {
            let parts: Vec<&str> = l.splitn(5, '|').collect();
            if parts.len() < 4 {
                continue;
            }
            let name = parts[0].to_string();
            let pid = parts[1].parse::<i32>().unwrap_or(-1);
            let dir = parts[2].to_string();
            let cmd_str = parts[3];
            let cmd_parts: Vec<String> = cmd_str.split('\t').map(|s| s.to_string()).collect();
            let timestamp = if parts.len() >= 5 {
                parts[4].parse::<u64>().unwrap_or(0)
            } else {
                0
            };
            let e = ContainerEntry { name, pid, dir, command: cmd_parts, timestamp };
            v.push(e);
        }
    }
    v
}

pub fn get_process_uptime(pid: i32) -> Result<u64, std::io::Error> {
    let p = format!("/proc/{}/stat", pid);
    let mut b = String::new();
    File::open(&p)?.read_to_string(&mut b)?;
    let f: Vec<&str> = b.split_whitespace().collect();
    if f.len() <= 21 {
        return Err(std::io::Error::new(std::io::ErrorKind::Other, "Failed to parse stat"));
    }
    let st: f64 = f[21].parse().unwrap_or(0.0);
    let mut us = String::new();
    File::open("/proc/uptime")?.read_to_string(&mut us)?;
    let su: f64 = us.split_whitespace().next().unwrap_or("0").parse().unwrap_or(0.0);
    let hz = 100.0;
    let ss = st / hz;
    let pu = su - ss;
    Ok(pu.max(0.0) as u64)
}
