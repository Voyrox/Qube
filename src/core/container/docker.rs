use std::fs;
use std::env;
use serde_yaml::{Mapping, Value};
use crate::core::container::custom::{read_qube_yaml, CommandValue, ENVValue};
use serde_json;

pub fn convert_and_run(dockerfile_path: &str) {
    let content = fs::read_to_string(dockerfile_path)
        .unwrap_or_else(|_| panic!("Could not read Dockerfile at {}", dockerfile_path));

    let mut from_line: Option<String> = None;
    let mut ports: Vec<String> = Vec::new();
    let mut cmd_line: Option<String> = None;
    let mut installation_commands: Vec<String> = Vec::new();

    for line in content.lines() {
        let trimmed = line.trim();
        match trimmed.split_whitespace().next() {
            Some("FROM") => {
                from_line = trimmed.split_whitespace().nth(1).map(String::from)
            },
            Some("EXPOSE") => {
                ports.extend(trimmed.split_whitespace().skip(1).map(String::from))
            },
            Some("CMD") => {
                cmd_line = Some(trimmed.trim_start_matches("CMD").trim().to_string());
            },
            Some("ENV") if trimmed.contains("INSTALL_") => {
                if trimmed.contains("INSTALL_NODE=true") {
                    installation_commands.push("curl -fsSL https://deb.nodesource.com/setup_23.x | bash -".to_string());
                    installation_commands.push("apt-get install -y nodejs".to_string());
                }
                if trimmed.contains("INSTALL_RUST=true") {
                    installation_commands.push("curl https://sh.rustup.rs -sSf | sh -s -- -y".to_string());
                }
                if trimmed.contains("INSTALL_PYTHON=true") {
                    installation_commands.push("apt-get install -y python3".to_string());
                }
                if trimmed.contains("INSTALL_GOLANG=true") {
                    installation_commands.push("apt-get install -y golang".to_string());
                }
                if trimmed.contains("INSTALL_JAVA=true") {
                    installation_commands.push("apt-get install -y default-jdk".to_string());
                }
            },
            _ => {}
        }
    }

    let from_value = from_line.expect("Dockerfile does not contain a FROM instruction.");
    let cmd_value = cmd_line.expect("Dockerfile does not contain a CMD instruction.");

    let system = if from_value.to_lowercase().contains("node") {
        "Ubuntu24_NODE".to_string()
    } else {
        from_value.clone()
    };

    let mut cmd_list = installation_commands;
    if cmd_value.starts_with('[') && cmd_value.ends_with(']') {
        match serde_json::from_str::<Vec<String>>(&cmd_value) {
            Ok(mut cmds) => {
                cmd_list.append(&mut cmds);
            },
            Err(_) => {
                cmd_list.push(cmd_value);
            }
        }
    } else {
        cmd_list.push(cmd_value);
    }

    let mut container_map = Mapping::new();
    container_map.insert(Value::String("system".to_string()), Value::String(system));
    if !ports.is_empty() {
        let port_values: Vec<Value> = ports.into_iter().map(Value::String).collect();
        container_map.insert(Value::String("ports".to_string()), Value::Sequence(port_values));
    }
    let cmd_values: Vec<Value> = cmd_list.into_iter().map(Value::String).collect();
    container_map.insert(Value::String("cmd".to_string()), Value::Sequence(cmd_values));
    container_map.insert(Value::String("isolated".to_string()), Value::Bool(false));
    container_map.insert(Value::String("debug".to_string()), Value::Bool(false));

    let mut top_level = Mapping::new();
    top_level.insert(Value::String("container".to_string()), Value::Mapping(container_map));

    let yaml_str = serde_yaml::to_string(&top_level)
        .expect("Failed to convert configuration to YAML");

    fs::write("qube.yml", yaml_str)
        .expect("Failed to write qube.yml");
    println!("Converted Dockerfile to qube.yml successfully.");

    let cwd = env::current_dir()
        .expect("Failed to get current directory")
        .to_string_lossy()
        .to_string();
    let config = read_qube_yaml().expect("Error reading qube.yml");
    let ports_str = config.ports.map(|p| p.join(",")).unwrap_or_else(|| "".to_string());
    let isolated = config.isolated.unwrap_or(false);
    let volumes: Vec<(String, String)> = config.volumes.unwrap_or_default()
        .into_iter()
        .map(|v| (v.host_path, v.container_path))
        .collect();
    let image = config.system.trim().to_string();

    crate::core::container::validate_image(&image).expect("Invalid image provided");

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
        &ports_str,
        isolated,
        &volumes,
        &env_vars,
    );
    eprintln!(
        "\nContainer {} built from Dockerfile conversion. It will be started by the daemon.",
        container_id
    );
}
