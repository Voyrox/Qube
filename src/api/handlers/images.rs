use std::fs;
use std::path::Path;
use warp::{Rejection, Reply};
use serde::{Deserialize, Serialize};
use crate::config::QUBE_CONTAINERS_BASE;
use crate::core::tracking;

#[derive(Debug, Serialize, Deserialize)]
pub struct ImageInfo {
    pub name: String,
    pub size_mb: f64,
    pub path: String,
}

pub async fn list_images() -> Result<impl Reply, Rejection> {
    let images_dir = format!("{}/images", QUBE_CONTAINERS_BASE);
    let path = Path::new(&images_dir);
    
    let mut images = Vec::new();
    
    if path.exists() && path.is_dir() {
        if let Ok(entries) = fs::read_dir(path) {
            for entry in entries.flatten() {
                if let Ok(metadata) = entry.metadata() {
                    if metadata.is_file() {
                        let file_name = entry.file_name().to_string_lossy().to_string();
                        let size_bytes = metadata.len();
                        let size_mb = size_bytes as f64 / 1_048_576.0; // Convert to MB
                        
                        images.push(ImageInfo {
                            name: file_name,
                            size_mb,
                            path: entry.path().to_string_lossy().to_string(),
                        });
                    }
                }
            }
        }
    }
    
    Ok(warp::reply::json(&images))
}

#[derive(Debug, Serialize, Deserialize)]
pub struct VolumeInfo {
    pub name: String,
    pub host_path: String,
    pub container_path: String,
    pub container: String,
}

pub async fn list_volumes() -> Result<impl Reply, Rejection> {
    // Get volumes from tracked containers
    let containers = tracking::get_all_tracked_entries();
    let mut volumes = Vec::new();
    
    for container in containers {
        for (idx, (host_path, container_path)) in container.volumes.iter().enumerate() {
            volumes.push(VolumeInfo {
                name: format!("vol-{}", idx),
                host_path: host_path.clone(),
                container_path: container_path.clone(),
                container: container.name.clone(),
            });
        }
    }
    
    Ok(warp::reply::json(&volumes))
}
