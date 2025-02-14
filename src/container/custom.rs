use std::fs;
use std::path::Path;
use serde::Deserialize;

#[derive(Deserialize)]
pub struct Config {
    pub system: String,
    pub cmd: String,
    pub ports: Option<String>,
    pub isolated: Option<bool>,
    pub debug: Option<bool>,
}

pub fn read_qube_yaml() -> Result<Config, Box<dyn std::error::Error>> {
    let path = Path::new("./qube.yml");
    if !path.exists() {
        return Err("qube.yml file not found in the current directory".into());
    }
    let yaml_str = fs::read_to_string(path)?;
    let config: Config = serde_yaml::from_str(&yaml_str)?;
    Ok(config)
}