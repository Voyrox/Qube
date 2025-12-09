use colored::*;
use std::env;
use std::path::Path;
use std::process::exit;

use crate::core::container::custom::{CommandValue, ENVValue};

pub fn run_command(args: &[String]) {
    if let Some(cmd_flag_index) = args.iter().position(|arg| arg == "--cmd") {
        let mut image = None;
        let mut ports = "".to_string();
        let mut isolated = false;
        let mut volumes: Vec<(String, String)> = Vec::new();
        let mut env_vars: Vec<String> = Vec::new();

        let mut i = 2;
        while i < cmd_flag_index {
            match args[i].as_str() {
                "--image" => {
                    if let Some(val) = args.get(i + 1) {
                        image = Some(val.clone());
                        i += 2;
                        continue;
                    } else {
                        eprintln!("{}", "Usage: qube run [--image <image>] [--ports <ports>] [--env <NAME=VALUE>] [--volume /host/path:/container/path] [--isolated] [--debug] --cmd \"<command>\"".bright_red());
                        exit(1);
                    }
                }
                "--ports" => {
                    if let Some(val) = args.get(i + 1) {
                        ports = val.clone();
                        i += 2;
                        continue;
                    } else {
                        eprintln!("{}", "Usage: qube run [--image <image>] [--ports <ports>] [--env <NAME=VALUE>] [--volume /host/path:/container/path] [--isolated] [--debug] --cmd \"<command>\"".bright_red());
                        exit(1);
                    }
                }
                "--isolated" => {
                    isolated = true;
                    i += 1;
                    continue;
                }
                "--volume" => {
                    if let Some(val) = args.get(i + 1) {
                        let parts: Vec<&str> = val.splitn(2, ':').collect();
                        if parts.len() != 2 {
                            eprintln!("Error: --volume argument must be in the format /host/path:/container/path");
                            exit(1);
                        }
                        volumes.push((parts[0].to_string(), parts[1].to_string()));
                        i += 2;
                        continue;
                    } else {
                        eprintln!("Usage: qube run [--volume <host_path>:<container_path>] ...");
                        exit(1);
                    }
                }
                "--env" => {
                    if let Some(val) = args.get(i + 1) {
                        if val.contains('=') {
                            env_vars.push(val.clone());
                            i += 2;
                            continue;
                        } else {
                            eprintln!("Error: --env argument must be in the format KEY=VALUE");
                            exit(1);
                        }
                    } else {
                        eprintln!("Usage: qube run [--env KEY=VALUE] ...");
                        exit(1);
                    }
                }
                _ => {
                    i += 1;
                }
            }
        }

        let image = match image {
            Some(img) => img,
            None => {
                eprintln!("{}", "Error: --image flag must be specified.".bright_red());
                exit(1);
            }
        };

        if let Err(e) = crate::core::container::validate_image(&image) {
            eprintln!(
                "{}",
                format!("Error: Invalid image provided ('{}'). Reason: {}", image, e)
                    .bright_red()
                    .bold()
            );
            exit(1);
        }

        let user_cmd: Vec<String> = args[cmd_flag_index + 1..].to_vec();
        let cwd = env::current_dir().map(|dir| dir.to_string_lossy().to_string()).unwrap_or_else(|e| {
            eprintln!("{}", format!("Failed to get current directory: {}", e).bright_red());
            exit(1);
        });

        let container_id = crate::core::container::lifecycle::build_container(None, &cwd, &image);

        crate::core::tracking::track_container_named(
            &container_id,
            -1,
            &cwd,
            user_cmd.clone(),
            &image,
            &ports,
            isolated,
            &volumes,
            &env_vars
        );
        eprintln!(
            "{}",
            format!("\nContainer {} built. It will be started by the daemon.", container_id)
                .green()
                .bold()
        );
    } else if Path::new("./qube.yml").exists() {
        println!("{}", "Detected qube.yml configuration file. Building container based on YAML configuration.".green().bold());

        let config = match crate::core::container::custom::read_qube_yaml() {
            Ok(cfg) => cfg,
            Err(e) => {
                eprintln!("Error reading qube.yml: {}", e);
                exit(1);
            }
        };

        let ports = config.ports.map(|ports| ports.join(",")).unwrap_or_else(|| "".to_string());
        let isolated = config.isolated.unwrap_or(false);
        let _debug = config.debug.unwrap_or(false);
        let volumes: Vec<(String, String)> = config.volumes.unwrap_or_default().into_iter()
            .map(|v| (v.host_path, v.container_path))
            .collect();
        let image = if config.system.trim().is_empty() {
            eprintln!("{}", "Error: 'system' field in qube.yml is an invalid image.".bright_red());
            exit(1);
        } else {
            config.system
        };

        let cwd = env::current_dir().map(|dir| dir.to_string_lossy().to_string()).unwrap_or_else(|e| {
            eprintln!("{}", format!("Failed to get current directory: {}", e).bright_red());
            exit(1);
        });

        if let Err(e) = crate::core::container::validate_image(&image) {
            eprintln!(
                "{}",
                format!("Error: Invalid image provided ('{}'). Reason: {}", image, e)
                    .bright_red()
                    .bold()
            );
            exit(1);
        }

        let container_id = crate::core::container::lifecycle::build_container(None, &cwd, &image);

        let command_str = match config.cmd {
            CommandValue::Single(s) => s,
            CommandValue::List(l) => l.join(" && "),
        };
        let cmd_vec = vec![command_str];

        let env_vars = match config.enviroment {
            Some(ENVValue::Single(s)) => vec![s],
            Some(ENVValue::List(l)) => l,
            None => vec![],
        };

        crate::core::tracking::track_container_named(
            &container_id,
            -1,
            &cwd,
            cmd_vec,
            &image,
            &ports,
            isolated,
            &volumes,
            &env_vars
        );

        eprintln!(
            "{}",
            format!("\nContainer {} built from YAML configuration. It will be started by the daemon.", container_id)
                .green()
                .bold()
        );
    } else {
        eprintln!("{}", "Usage: qube run [--image <image>] [--ports <ports>] [--env <NAME=VALUE>] [--volume /host/path:/container/path] [--isolated] [--debug] --cmd \"<command>\" OR a qube.yml file must be present in the current directory.".bright_red());
        exit(1);
    }
}
