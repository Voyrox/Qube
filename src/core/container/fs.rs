use std::fs;
use std::path::Path;
use std::process::Command;
use nix::mount::{mount, MsFlags, umount2, MntFlags};
use crate::config::QUBE_CONTAINERS_BASE;

pub fn mount_volume(cid: &str, host_path: &str, container_path: &str) -> Result<(), std::io::Error> {
    if !Path::new(host_path).exists() {
        return Err(std::io::Error::new(std::io::ErrorKind::NotFound,
            format!("Host path '{}' does not exist", host_path)));
    }

    let rootfs = get_rootfs(cid);
    let dest = format!("{}/{}", rootfs, container_path.trim_start_matches('/'));

    if !Path::new(&dest).exists() {
        println!("DEBUG: Creating mount point at {}", dest);
        fs::create_dir_all(&dest)?;
    }    

    println!("DEBUG: Attempting to mount {} -> {}", host_path, dest);

    let _ = umount2(Path::new(&dest), MntFlags::MNT_DETACH);

    let rootfs_path = std::path::Path::new(&rootfs);
    let _ = mount(
        Some(rootfs_path),
        rootfs_path,
        None::<&str>,
        MsFlags::MS_REC | MsFlags::MS_SHARED,
        None::<&str>,
    );
    Ok(())
}

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
