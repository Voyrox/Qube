use serde::Deserialize;

#[derive(Deserialize)]
#[serde(untagged)]
pub enum CommandValue {
    Single(String),
    List(Vec<String>),
}

#[derive(Deserialize)]
pub struct ContainerConfig {
    pub system: String,
    pub cmd: CommandValue,
    pub ports: Option<Vec<String>>,
    pub isolated: Option<bool>,
    pub debug: Option<bool>,
}

#[derive(Deserialize)]
struct QubeConfig {
    container: ContainerConfig,
}

pub fn read_qube_yaml() -> Result<ContainerConfig, Box<dyn std::error::Error>> {
    let path = std::path::Path::new("./qube.yml");
    if !path.exists() {
        return Err("qube.yml file not found in the current directory".into());
    }
    let yaml_str = std::fs::read_to_string(path)?;
    let qube_config: QubeConfig = serde_yaml::from_str(&yaml_str)?;
    Ok(qube_config.container)
}