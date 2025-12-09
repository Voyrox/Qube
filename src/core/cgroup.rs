use std::fs;
use std::os::unix::fs::PermissionsExt;
use std::io;

use crate::config::{CGROUP_ROOT, MEMORY_MAX_MB, MEMORY_SWAP_MAX_MB};

pub const MEMORY_MAX: u64 = MEMORY_MAX_MB * 1024 * 1024;
pub const MEMORY_SWAP_MAX: u64 = MEMORY_SWAP_MAX_MB * 1024 * 1024;

/// Initialize the cgroup root directory (should be called once at daemon startup)
pub fn init_cgroup_root() -> io::Result<()> {
    // Create the main cgroup directory if it doesn't exist
    if !std::path::Path::new(CGROUP_ROOT).exists() {
        fs::create_dir_all(CGROUP_ROOT)?;
        fs::set_permissions(CGROUP_ROOT, fs::Permissions::from_mode(0o755))?;
        
        // Enable memory and cpu controllers
        let subtree_control = format!("{}/cgroup.subtree_control", CGROUP_ROOT);
        if let Err(e) = fs::write(&subtree_control, "+memory +cpu") {
            eprintln!("Warning: Failed to enable cgroup controllers: {}", e);
        }
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
    
    // Set CPU limits (optional - can add CPU quota/weight here)
    // For now, we'll just ensure the CPU controller is available
    
    Ok(cgroup_path)
}

/// Add a process to the container's cgroup
pub fn add_process_to_cgroup(cgroup_path: &str, pid: i32) -> io::Result<()> {
    let cgroup_procs = format!("{}/cgroup.procs", cgroup_path);
    fs::write(&cgroup_procs, pid.to_string())?;
    Ok(())
}

/// Remove a container's cgroup when it's deleted
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