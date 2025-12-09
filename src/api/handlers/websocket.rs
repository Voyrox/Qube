use warp::Filter;
use warp::ws::{Message, WebSocket};
use futures::StreamExt;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use futures::SinkExt;

pub async fn eval_ws(ws: WebSocket, _container_identifier: String, _command_to_run: String) {
    let (mut ws_tx, mut ws_rx) = ws.split();

    let tracked = crate::core::tracking::get_all_tracked_entries();
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
