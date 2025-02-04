use std::fs;
use std::io::{Read, Write};
use std::fs::File;

pub const TRACKING_DIR: &str = "/var/lib/Qube";
pub const CONTAINER_LIST_FILE: &str = "/var/lib/Qube/containers.txt";

#[derive(Debug)]
pub struct ContainerEntry {
    pub name: String,
    pub pid: i32,
    pub dir: String,
    pub command: Vec<String>,
}

pub fn track_container_named(n: &str, p: i32, d: &str, c: Vec<String>) {
    fs::create_dir_all(TRACKING_DIR).ok();
    let s = c.join("\t");
    let line = format!("{}|{}|{}|{}", n, p, d, s);
    let mut f = fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(CONTAINER_LIST_FILE)
        .expect("Failed to open container tracking file");
    let _ = writeln!(f, "{}", line);
}

pub fn remove_container_from_tracking(pid: i32) {
    if let Ok(c) = fs::read_to_string(CONTAINER_LIST_FILE) {
        let mut nc = Vec::new();
        for l in c.lines() {
            let parts: Vec<&str> = l.splitn(4, '|').collect();
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
        let _ = fs::write(CONTAINER_LIST_FILE, joined);
    }
}

pub fn remove_container_from_tracking_by_name(name: &str) {
    if let Ok(c) = fs::read_to_string(CONTAINER_LIST_FILE) {
        let new_lines: Vec<String> = c
            .lines()
            .filter(|l| {
                let parts: Vec<&str> = l.splitn(4, '|').collect();
                if parts.len() < 4 {
                    return true;
                }
                parts[0] != name
            })
            .map(String::from)
            .collect();
        let joined = new_lines.join("\n");
        let _ = fs::write(CONTAINER_LIST_FILE, joined);
    }
}

pub fn get_all_tracked_entries() -> Vec<ContainerEntry> {
    let mut v = Vec::new();
    if let Ok(c) = fs::read_to_string(CONTAINER_LIST_FILE) {
        for l in c.lines() {
            let parts: Vec<&str> = l.splitn(4, '|').collect();
            if parts.len() < 4 {
                continue;
            }
            let name = parts[0].to_string();
            let pid = parts[1].parse::<i32>().unwrap_or(-1);
            let dir = parts[2].to_string();
            let cmd_str = parts[3];
            let cmd_parts: Vec<String> = cmd_str.split('\t').map(|s| s.to_string()).collect();
            let e = ContainerEntry { name, pid, dir, command: cmd_parts };
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
