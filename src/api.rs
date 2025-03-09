use warp::Filter;
use serde::{Deserialize, Serialize};
use crate::container::lifecycle;
use crate::tracking;
use std::path::Path;
use warp::ws::{Message, WebSocket};
use futures::StreamExt;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use futures::SinkExt;

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

pub async fn eval_ws(ws: WebSocket, _container_identifier: String, _command_to_run: String) {
    let (mut ws_tx, mut ws_rx) = ws.split();

    let tracked = crate::tracking::get_all_tracked_entries();
    let entry_opt = tracked.iter().find(|e| {
        e.name == _container_identifier || e.pid.to_string() == _container_identifier
    });

    if let Some(entry) = entry_opt {
        let nsenter_cmd = format!(
            "nsenter -t {} -a env TERM=dumb script -q -c '/bin/bash -i' /dev/null",
            entry.pid
        );

        let mut child = tokio::process::Command::new("sh")
            .arg("-c")
            .arg(nsenter_cmd)
            .stdin(std::process::Stdio::piped())
            .stdout(std::process::Stdio::piped())
            .spawn()
            .expect("Failed to execute nsenter command");

        let mut child_stdout = child.stdout.take().expect("Failed to open stdout");
        let mut child_stdin = child.stdin.take().expect("Failed to open stdin");

        tokio::spawn(async move {
            while let Some(result) = ws_rx.next().await {
                match result {
                    Ok(msg) if msg.is_text() => {
                        let mut input = msg.to_str().unwrap().to_string();
                        if !input.ends_with('\n') {
                            input.push('\n');
                        }
                        if let Err(e) = child_stdin.write_all(input.as_bytes()).await {
                            eprintln!("Failed to write to child_stdin: {}", e);
                            break;
                        }
                    }
                    Ok(_) => {}
                    Err(e) => {
                        eprintln!("WebSocket error: {}", e);
                        break;
                    }
                }
            }
        });

        tokio::spawn(async move {
            let mut buf = [0; 1024];
            loop {
                match child_stdout.read(&mut buf).await {
                    Ok(n) if n > 0 => {
                        let output = String::from_utf8_lossy(&buf[..n]);
                        if let Err(e) = ws_tx.send(Message::text(output)).await {
                            eprintln!("Failed to send to WebSocket: {}", e);
                            break;
                        }
                    }
                    Ok(_) => break,
                    Err(e) => {
                        eprintln!("Failed to read from child_stdout: {}", e);
                        break;
                    }
                }
            }
        });

        child.wait().await.expect("Child process wasn't running");
    } else {
        ws_tx.send(Message::text("Container not found")).await.unwrap();
    }
}

pub fn eval_ws_filter() -> impl Filter<Extract = impl warp::Reply, Error = warp::Rejection> + Clone {
    warp::path("eval")
        .and(warp::ws())
        .and(warp::path::param())
        .and(warp::path::param())
        .and_then(|ws: warp::ws::Ws, container_identifier: String, command_to_run: String| async move {
            Ok::<_, warp::Rejection>(
                ws.on_upgrade(move |socket| eval_ws(socket, container_identifier, command_to_run))
            )
        })
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

    let eval_ws_route = eval_ws_filter();

    let routes = eval_ws_route
        .or(list)
        .or(stop)
        .or(start)
        .or(delete)
        .or(info)
        .with(warp::cors().allow_any_origin());

    tokio::runtime::Runtime::new().unwrap().block_on(async {
        println!("API server is running at http://127.0.0.1:3030");
        warp::serve(routes)
            .run(([127, 0, 0, 1], 3030))
            .await;
    });
}