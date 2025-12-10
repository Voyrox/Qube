use serde::{Deserialize, Serialize};
use crate::core::container::lifecycle;
use crate::core::tracking;
use crate::core::cgroup;
use std::path::Path;

#[derive(Deserialize)]
pub struct CommandParams {
    pub container_id: String,
}

#[derive(Serialize)]
pub struct Response {
    pub containers: Vec<ContainerInfo>,
}

#[derive(Serialize)]
pub struct ContainerInfo {
    pub name: String,
    pub pid: i32,
    pub directory: String,
    pub command: Vec<String>,
    pub image: String,
    pub timestamp: u64,
    pub ports: String,
    pub isolated: bool,
    pub volumes: Vec<(String, String)>,
    pub enviroment: Vec<String>,
    pub memory_mb: Option<f64>,
    pub cpu_percent: Option<f64>,
}

#[derive(Debug)]
pub enum ContainerError {
    InvalidContainerId,
    ContainerNotFound,
}

impl warp::reject::Reject for ContainerError {}

pub async fn list_containers() -> Result<impl warp::Reply, warp::Rejection> {
    let entries = tracking::get_all_tracked_entries();
    let response = if entries.is_empty() {
        Vec::new()
    } else {
        let containers: Vec<ContainerInfo> = entries.into_iter().map(|entry| {
            let memory_mb = if entry.pid > 0 {
                let mem_bytes = cgroup::get_memory_stats(&entry.name)
                    .ok()
                    .map(|stats| stats.current_bytes)
                    .unwrap_or(0);
                
                if mem_bytes > 0 {
                    Some(mem_bytes as f64 / (1024.0 * 1024.0))
                } else {
                    cgroup::get_memory_from_proc(entry.pid)
                        .ok()
                        .filter(|&bytes| bytes > 0)
                        .map(|bytes| bytes as f64 / (1024.0 * 1024.0))
                }
            } else {
                None
            };
            
            let cpu_percent = if entry.pid > 0 {
                cgroup::get_cpu_from_proc(entry.pid).ok()
            } else {
                None
            };
            
            ContainerInfo {
                name: entry.name.clone(),
                pid: entry.pid,
                directory: entry.dir.clone(),
                command: entry.command.clone(),
                image: entry.image.clone(),
                timestamp: entry.timestamp.clone(),
                ports: entry.ports.clone(),
                isolated: entry.isolated,
                volumes: entry.volumes.clone(),
                enviroment: entry.env_vars.clone(),
                memory_mb,
                cpu_percent,
            }
        }).collect();
        containers
    };

    Ok(warp::reply::json(&Response {
        containers: response,
    }))
}

pub async fn stop_container(params: CommandParams) -> Result<impl warp::Reply, warp::Rejection> {
    if let Ok(pid) = params.container_id.parse::<i32>() {
        lifecycle::stop_container(pid);
        Ok(warp::reply::json(&Response {
            containers: vec![ContainerInfo {
                name: "Container stopped".to_string(),
                pid,
                directory: "".to_string(),
                command: Vec::new(),
                image: "".to_string(),
                timestamp: 0,
                ports: "".to_string(),
                isolated: false,
                volumes: Vec::new(),
                enviroment: Vec::new(),
                memory_mb: None,
                cpu_percent: None,
            }],
        }))
    } else {
        Err(warp::reject::custom(ContainerError::InvalidContainerId))
    }
}

pub async fn start_container(params: CommandParams) -> Result<impl warp::Reply, warp::Rejection> {
    let pid: i32 = params.container_id.parse().unwrap_or(-1);
    if pid == -1 {
        return Err(warp::reject::custom(ContainerError::InvalidContainerId));
    }

    let tracked = tracking::get_all_tracked_entries();
    let entry_opt = tracked.iter().find(|e| {
        e.name == params.container_id || e.pid == pid
    });

    if let Some(entry) = entry_opt {
        if entry.pid > 0 && Path::new(&format!("/proc/{}", entry.pid)).exists() {
            return Ok(warp::reply::json(&Response {
                containers: vec![ContainerInfo {
                    name: entry.name.clone(),
                    pid: entry.pid,
                    directory: entry.dir.clone(),
                    command: entry.command.clone(),
                    image: entry.image.clone(),
                    timestamp: entry.timestamp.clone(),
                    ports: entry.ports.clone(),
                    isolated: entry.isolated,
                    volumes: entry.volumes.clone(),
                    enviroment: entry.env_vars.clone(),
                    memory_mb: cgroup::get_memory_stats(&entry.name).ok().map(|s| s.current_mb()),
                    cpu_percent: cgroup::get_cpu_from_proc(entry.pid).ok(),
                }],
            }));
        } else if entry.pid == -1 {
            return Ok(warp::reply::json(&Response {
                containers: vec![ContainerInfo {
                    name: entry.name.clone(),
                    pid: entry.pid,
                    directory: entry.dir.clone(),
                    command: entry.command.clone(),
                    image: entry.image.clone(),
                    timestamp: entry.timestamp.clone(),
                    ports: entry.ports.clone(),
                    isolated: entry.isolated,
                    volumes: entry.volumes.clone(),
                    enviroment: entry.env_vars.clone(),
                    memory_mb: None,
                cpu_percent: None,
                }],
            }));
        } else {
            tracking::update_container_pid(
                &entry.name,
                -1,
                &entry.dir,
                &entry.command,
                &entry.image,
                &entry.ports,
                entry.isolated,
                &entry.volumes,
                &entry.env_vars,
            );
            return Ok(warp::reply::json(&Response {
                containers: vec![ContainerInfo {
                    name: entry.name.clone(),
                    pid: entry.pid,
                    directory: entry.dir.clone(),
                    command: entry.command.clone(),
                    image: entry.image.clone(),
                    timestamp: entry.timestamp.clone(),
                    ports: entry.ports.clone(),
                    isolated: entry.isolated,
                    volumes: entry.volumes.clone(),
                    enviroment: entry.env_vars.clone(),
                    memory_mb: None,
                cpu_percent: None,
                }],
            }));
        }
    }

    Ok(warp::reply::json(&Response {
        containers: vec![ContainerInfo {
            name: "Container not found".to_string(),
            pid: -1,
            directory: "".to_string(),
            command: Vec::new(),
            image: "".to_string(),
            timestamp: 0,
            ports: "".to_string(),
            isolated: false,
            volumes: Vec::new(),
            enviroment: Vec::new(),
            memory_mb: None,
                cpu_percent: None,
        }],
    }))
}

pub async fn delete_container(params: CommandParams) -> Result<impl warp::Reply, warp::Rejection> {
    let pid: i32 = params.container_id.parse().unwrap_or(-1);
    if pid == -1 {
        return Err(warp::reject::custom(ContainerError::InvalidContainerId));
    }
    lifecycle::kill_container(pid);
    Ok(warp::reply::json(&Response {
        containers: vec![ContainerInfo {
            name: "Killed container".to_string(),
            pid,
            directory: "".to_string(),
            command: Vec::new(),
            image: "".to_string(),
            timestamp: 0,
            ports: "".to_string(),
            isolated: false,
            volumes: Vec::new(),
            enviroment: Vec::new(),
            memory_mb: None,
                cpu_percent: None,
        }],
    }))
}

pub async fn container_info(params: CommandParams) -> Result<impl warp::Reply, warp::Rejection> {
    let tracked = tracking::get_all_tracked_entries();
    let entry = tracked.iter().find(|e| e.pid == params.container_id.parse().unwrap_or(-1));

    if let Some(entry) = entry {
        Ok(warp::reply::json(&Response {
            containers: vec![ContainerInfo {
                name: entry.name.clone(),
                pid: entry.pid,
                directory: entry.dir.clone(),
                command: entry.command.clone(),
                image: entry.image.clone(),
                timestamp: entry.timestamp.clone(),
                ports: entry.ports.clone(),
                isolated: entry.isolated,
                volumes: entry.volumes.clone(),
                enviroment: entry.env_vars.clone(),
                memory_mb: if entry.pid > 0 { cgroup::get_memory_stats(&entry.name).ok().map(|s| s.current_mb()) } else { None },
                cpu_percent: if entry.pid > 0 { cgroup::get_cpu_from_proc(entry.pid).ok() } else { None },
            }],
        }))
    } else {
        Err(warp::reject::custom(ContainerError::ContainerNotFound))
    }
}
