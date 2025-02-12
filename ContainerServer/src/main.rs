use tokio_util::io::ReaderStream;
use warp::{Filter, Reply};
use warp::hyper::Body;

#[tokio::main]
async fn main() {
    if let Err(e) = tokio::fs::create_dir_all("./files").await {
        eprintln!("Failed to create files directory: {}", e);
        std::process::exit(1);
    }

    let download_route = warp::path("files")
        .and(warp::path::param::<String>())
        .and_then(serve_file);

    println!("Server started at 0.0.0.0:8080");

    let (_addr, server) = warp::serve(download_route)
        .bind_with_graceful_shutdown(([0, 0, 0, 0], 8080), shutdown_signal());
    server.await;
}

async fn serve_file(file_name: String) -> Result<impl Reply, warp::Rejection> {
    let file_path = format!("./files/{}", file_name);

    let file = tokio::fs::File::open(&file_path).await.map_err(|e| {
        eprintln!("Error opening file {}: {}", file_path, e);
        warp::reject::not_found()
    })?;

    let metadata = tokio::fs::metadata(&file_path).await.map_err(|e| {
        eprintln!("Error getting metadata for {}: {}", file_path, e);
        warp::reject::not_found()
    })?;
    let file_size = metadata.len();

    let stream = ReaderStream::new(file);
    let body = Body::wrap_stream(stream);

    let mut response = warp::reply::Response::new(body);
    let content_type = if file_name.ends_with(".pdf") {
        "application/pdf"
    } else if file_name.ends_with(".png") {
        "image/png"
    } else if file_name.ends_with(".tar") {
        "application/x-tar"
    } else {
        "application/octet-stream"
    };

    response.headers_mut().insert("Content-Type", content_type.parse().unwrap());
    response.headers_mut().insert("Content-Length", file_size.to_string().parse().unwrap());

    Ok(response)
}

async fn shutdown_signal() {
    #[cfg(unix)]
    {
        use tokio::signal::unix::{signal, SignalKind};
        let mut sigterm = signal(SignalKind::terminate())
            .expect("failed to set up SIGTERM handler");
        let mut sigint = signal(SignalKind::interrupt())
            .expect("failed to set up SIGINT handler");

        tokio::select! {
            _ = sigterm.recv() => {},
            _ = sigint.recv() => {},
        }
    }

    #[cfg(not(unix))]
    {
        tokio::signal::ctrl_c()
            .await
            .expect("failed to listen for Ctrl+C");
    }

    println!("Shutdown signal received. Stopping server...");
}
