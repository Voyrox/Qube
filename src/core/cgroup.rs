use std::fs;
use std::os::unix::fs::PermissionsExt;
use std::io;

use crate::config::{CGROUP_ROOT, MEMORY_MAX_MB, MEMORY_SWAP_MAX_MB, CPU_QUOTA_US, CPU_PERIOD_US};

pub const MEMORY_MAX: u64 = MEMORY_MAX_MB * 1024 * 1024;
pub const MEMORY_SWAP_MAX: u64 = MEMORY_SWAP_MAX_MB * 1024 * 1024;

/// Initialize the cgroup root directory (should be called once at daemon startup)
pub fn init_cgroup_root() -> io::Result<()> {
    // Create the main cgroup directory if it doesn't exist
    if !std::path::Path::new(CGROUP_ROOT).exists() {
        fs::create_dir_all(CGROUP_ROOT)?;
        fs::set_permissions(CGROUP_ROOT, fs::Permissions::from_mode(0o755))?;
    }
    
    // Always try to enable memory, cpu, and net_cls controllers (even if dir already exists)
    let subtree_control = format!("{}/cgroup.subtree_control", CGROUP_ROOT);
    if let Err(e) = fs::write(&subtree_control, "+memory +cpu") {
        eprintln!("Warning: Failed to enable cgroup controllers: {}", e);
    } else {
        eprintln!("✓ Cgroup controllers enabled: memory, cpu");
    }
    
    Ok(())
}

/// Setup cgroup for a specific container (called from parent process before fork)
pub fn setup_cgroup_for_container(container_name: &str) -> io::Result<String> {
    init_cgroup_root()?;
    
    let cgroup_path = format!("{}/{}", CGROUP_ROOT, container_name);
    
    // Create container-specific cgroup directory
    if !std::path::Path::new(&cgroup_path).exists() {
        fs::create_dir_all(&cgroup_path)?;
        fs::set_permissions(&cgroup_path, fs::Permissions::from_mode(0o755))?;
    }
    
    // Set memory limits
    let mem_max_path = format!("{}/memory.max", cgroup_path);
    let mem_swap_path = format!("{}/memory.swap.max", cgroup_path);
    
    match fs::write(&mem_max_path, MEMORY_MAX.to_string()) {
        Ok(_) => {
            eprintln!("✓ Memory limit set: {} bytes ({} MB)", MEMORY_MAX, MEMORY_MAX_MB);
        }
        Err(e) => {
            eprintln!("Warning: Failed to set memory.max limit: {}", e);
        }
    }
    
    match fs::write(&mem_swap_path, MEMORY_SWAP_MAX.to_string()) {
        Ok(_) => {
            eprintln!("✓ Swap limit set: {} bytes ({} MB)", MEMORY_SWAP_MAX, MEMORY_SWAP_MAX_MB);
        }
        Err(e) => {
            eprintln!("Warning: Failed to set memory.swap.max limit: {}", e);
        }
    }
    
    // Set CPU limits
    let cpu_max_path = format!("{}/cpu.max", cgroup_path);
    let cpu_limit = format!("{} {}", CPU_QUOTA_US, CPU_PERIOD_US);
    
    match fs::write(&cpu_max_path, &cpu_limit) {
        Ok(_) => {
            eprintln!("✓ CPU limit set: {} cores max", CPU_QUOTA_US as f64 / CPU_PERIOD_US as f64);
        }
        Err(e) => {
            eprintln!("Warning: Failed to set cpu.max limit: {}", e);
        }
    }
    
    Ok(cgroup_path)
}

pub fn add_process_to_cgroup(cgroup_path: &str, pid: i32) -> io::Result<()> {
    let cgroup_procs = format!("{}/cgroup.procs", cgroup_path);
    fs::write(&cgroup_procs, pid.to_string())?;
    Ok(())
}

pub fn cleanup_cgroup(container_name: &str) -> io::Result<()> {
    let cgroup_path = format!("{}/{}", CGROUP_ROOT, container_name);
    if std::path::Path::new(&cgroup_path).exists() {
        fs::remove_dir(&cgroup_path)?;
    }
    Ok(())
}

/// Get memory usage statistics for a container
pub fn get_memory_stats(container_name: &str) -> io::Result<MemoryStats> {
    let cgroup_path = format!("{}/{}", CGROUP_ROOT, container_name);
    
    let current = fs::read_to_string(format!("{}/memory.current", cgroup_path))?
        .trim()
        .parse::<u64>()
        .unwrap_or(0);
    
    let max = fs::read_to_string(format!("{}/memory.max", cgroup_path))?
        .trim()
        .parse::<u64>()
        .unwrap_or(0);
    
    Ok(MemoryStats {
        current_bytes: current,
        max_bytes: max,
    })
}

pub fn get_memory_from_proc(pid: i32) -> io::Result<u64> {
    let status_path = format!("/proc/{}/status", pid);
    let status_content = fs::read_to_string(status_path)?;
    
    for line in status_content.lines() {
        if line.starts_with("VmRSS:") {
            let parts: Vec<&str> = line.split_whitespace().collect();
            if parts.len() >= 2 {
                if let Ok(kb) = parts[1].parse::<u64>() {
                    return Ok(kb * 1024); // Convert KB to bytes
                }
            }
        }
    }
    
    Ok(0)
}

/// Get CPU usage percentage from /proc for a specific PID
pub fn get_cpu_from_proc(pid: i32) -> io::Result<f64> {
    let stat_path = format!("/proc/{}/stat", pid);
    let stat_content = fs::read_to_string(stat_path)?;
    
    // Parse /proc/[pid]/stat to get utime + stime (CPU time in clock ticks)
    let parts: Vec<&str> = stat_content.split_whitespace().collect();
    if parts.len() >= 22 {
        // utime is at index 13, stime at index 14, starttime at index 21 (0-indexed)
        let utime = parts[13].parse::<u64>().unwrap_or(0);
        let stime = parts[14].parse::<u64>().unwrap_or(0);
        let starttime = parts[21].parse::<u64>().unwrap_or(0);
        let total_time = utime + stime;
        
        // Get system uptime and calculate process uptime
        let uptime_content = fs::read_to_string("/proc/uptime")?;
        let uptime_parts: Vec<&str> = uptime_content.split_whitespace().collect();
        if let Some(uptime_str) = uptime_parts.first() {
            if let Ok(system_uptime) = uptime_str.parse::<f64>() {
                // Convert everything to seconds (100 ticks/sec)
                let process_start_secs = starttime as f64 / 100.0;
                let process_uptime = system_uptime - process_start_secs;
                
                if process_uptime > 0.0 {
                    let cpu_secs = total_time as f64 / 100.0;
                    // CPU percent = (cpu_time / process_uptime) * 100
                    let cpu_percent = (cpu_secs / process_uptime) * 100.0;
                    return Ok(cpu_percent.min(400.0)); // Cap at 400% (4 cores)
                }
            }
        }
    }
    
    Ok(0.0)
}

#[derive(Debug)]
pub struct MemoryStats {
    pub current_bytes: u64,
    pub max_bytes: u64,
}

impl MemoryStats {
    pub fn current_mb(&self) -> f64 {
        self.current_bytes as f64 / (1024.0 * 1024.0)
    }
    
    pub fn max_mb(&self) -> f64 {
        self.max_bytes as f64 / (1024.0 * 1024.0)
    }
}