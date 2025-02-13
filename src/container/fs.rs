use std::fs;
use std::path::Path;
use std::process::Command;

pub const QUBE_CONTAINERS_BASE: &str = "/var/tmp/Qube_containers";

pub fn get_rootfs(cid: &str) -> String {
    format!("{}/rootfs", format!("{}/{}", QUBE_CONTAINERS_BASE, cid))
}

pub fn prepare_rootfs_dir(cid: &str) {
    let rootfs = get_rootfs(cid);
    if Path::new(&rootfs).exists() {
        fs::remove_dir_all(&rootfs).ok();
    }
    fs::create_dir_all(&rootfs).unwrap();
}

pub fn copy_directory_into_home(cid: &str, work_dir: &str) {
    let home_path = format!("{}/home", get_rootfs(cid));
    if !Path::new(&home_path).exists() {
        fs::create_dir_all(&home_path).ok();
    }
    let status = Command::new("cp")
        .args(["-r", &format!("{}/.", work_dir), &home_path])
        .status()
        .unwrap();
    if !status.success() {
        eprintln!("Warning: copying {} -> {} failed.", work_dir, home_path);
    }
}

pub fn mount_proc(cid: &str) -> Result<(), std::io::Error> {
    use nix::mount::{mount, MsFlags};
    let proc_path = format!("{}/proc", get_rootfs(cid));
    fs::create_dir_all(&proc_path)?;
    mount(
        Some("proc"),
        proc_path.as_str(),
        Some("proc"),
        MsFlags::MS_NOEXEC | MsFlags::MS_NOSUID | MsFlags::MS_NODEV,
        None::<&str>,
    )
    .map_err(|e| std::io::Error::new(std::io::ErrorKind::Other, e))
}
