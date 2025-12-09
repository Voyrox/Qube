pub fn docker_command(args: &[String]) {
    let dockerfile_path = if args.len() >= 3 {
        args[2].as_str()
    } else {
        "Dockerfile"
    };
    crate::core::container::docker::convert_and_run(dockerfile_path);
}
