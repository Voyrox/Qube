use warp::Filter;
use serde::{Deserialize, Serialize};
use crate::container::lifecycle;
use crate::tracking;
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

    let tracked = crate::tracking::get_all_tracked_entries();
    let entry_opt = tracked.iter().find(|e| {
        e.name == params.container_id || e.pid == pid // Compare pid directly
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
                }],
            }));
        } else {
            crate::tracking::update_container_pid(
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
            }],
        }))
    } else {
        Err(warp::reject::custom(ContainerError::ContainerNotFound))
    }
}

pub fn start_server() {
    let list = warp::path("list")
        .and(warp::get())
        .and_then(list_containers);

    let stop = warp::path("stop")
        .and(warp::post())
        .and(warp::body::json())
        .and_then(stop_container);

    let start = warp::path("start")
        .and(warp::post())
        .and(warp::body::json())
        .and_then(start_container);

    let delete = warp::path("delete")
        .and(warp::post())
        .and(warp::body::json())
        .and_then(delete_container);

    let info = warp::path("info")
        .and(warp::post())
        .and(warp::body::json())
        .and_then(container_info);

    let routes = list
        .or(stop)
        .or(start)
        .or(delete)
        .or(info);

    tokio::runtime::Runtime::new().unwrap().block_on(async {
        println!("API server is running at http://127.0.0.1:3030");
        warp::serve(routes)
            .run(([127, 0, 0, 1], 3030))
            .await;
    });
}