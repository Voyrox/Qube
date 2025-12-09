use warp::Filter;
use crate::api::handlers;

pub fn start_server() {
    let list = warp::path("list")
        .and(warp::get())
        .and_then(handlers::list_containers);

    let stop = warp::path("stop")
        .and(warp::post())
        .and(warp::body::json())
        .and_then(handlers::stop_container);

    let start = warp::path("start")
        .and(warp::post())
        .and(warp::body::json())
        .and_then(handlers::start_container);

    let delete = warp::path("delete")
        .and(warp::post())
        .and(warp::body::json())
        .and_then(handlers::delete_container);

    let info = warp::path("info")
        .and(warp::post())
        .and(warp::body::json())
        .and_then(handlers::container_info);

    let eval_ws_route = handlers::eval_ws_filter();

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
