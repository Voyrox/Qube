use std::fs;
use std::os::unix::fs::PermissionsExt;

pub const CGROUP_ROOT: &str = "/sys/fs/cgroup/QubeContainers";
pub const MEMORY_MAX: &str = "2147483648";
pub const MEMORY_SWAP_MAX: &str = "1073741824";

pub fn setup_cgroup2() -> i32 {
    use nix::unistd::getpid;
    let pid = getpid().as_raw();
    let cgroup_path = format!("{}/{}", CGROUP_ROOT, pid);
    
    if let Err(e) = fs::create_dir_all(&cgroup_path) {
        eprintln!("Failed to create cgroup dir: {}", e);
        return -1;
    }
    
    if let Err(e) = fs::set_permissions(&cgroup_path, fs::Permissions::from_mode(0o755)) {
        eprintln!("Failed to set permissions on cgroup directory: {}", e);
        return -1;
    }
    
    let mem_max_path = format!("{}/memory.max", cgroup_path);
    let mem_swap_path = format!("{}/memory.swap.max", cgroup_path);
    
    if fs::write(&mem_max_path, MEMORY_MAX).is_err() {
        eprintln!("Warning: Failed to set memory max limit.");
    }
    
    if fs::write(&mem_swap_path, MEMORY_SWAP_MAX).is_err() {
        eprintln!("Warning: Failed to set swap max limit.");
    }
    
    let cgroup_procs = format!("{}/cgroup.procs", cgroup_path);
    if let Err(e) = fs::write(&cgroup_procs, pid.to_string()) {
        eprintln!("Warning: Failed to write PID to cgroup.procs. Skipping cgroup. Error: {}", e);
        return -1;
    }
    
    pid
}

