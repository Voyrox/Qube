use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::io;
use reqwest::blocking::get;
use tar::Archive;
use flate2::read::GzDecoder;
use xz2::read::XzDecoder;
use serde::Deserialize;
use glob::glob;

/**
fn main() {
    let config = match read_qube_yaml() {
        Ok(cfg) => cfg,
        Err(e) => {
            eprintln!("Error reading qube.yml: {}", e);
            return;
        }
    };

    if config.system != "ubuntu" {
        eprintln!("Only Ubuntu systems are supported!");
        return;
    }

    let rootfs_path = "/mnt/c/Users/ewen2/Downloads/testrootfs";

    let container_path = match extract_ubuntu_container(rootfs_path) {
        Ok(path) => path,
        Err(e) => {
            eprintln!("Error extracting Ubuntu: {}", e);
            return;
        }
    };
    if let Err(e) = install_software(config.install, container_path.to_str().unwrap()) {
        eprintln!("Error installing software: {}", e);
        return;
    }

    if let Err(e) = run_cmd(&config.cmd) {
        eprintln!("Error running command: {}", e);
    }
}
 **/

#[derive(Deserialize)]
struct Config {
    system: String,
    install: Vec<String>,
    cmd: String,
}

fn read_qube_yaml() -> Result<Config, Box<dyn std::error::Error>> {
    let path = Path::new("./qube.yml");

    if !path.exists() {
        return Err("qube.yml file not found in the current directory".into());
    }

    println!("Reading configuration from {}", path.display());
    let yaml_str = fs::read_to_string(path)?;
    let config: Config = serde_yaml::from_str(&yaml_str)?;

    println!("Configuration loaded: system = {}, install = {:?}, cmd = {}",
             config.system, config.install, config.cmd);
    Ok(config)
}

fn download_file(url: &str, dest_path: &Path) -> Result<(), Box<dyn std::error::Error>> {
    println!("Downloading from {} to {}", url, dest_path.display());
    let mut response = get(url)?;
    let mut file = fs::File::create(dest_path)?;
    io::copy(&mut response, &mut file)?;
    println!("Download complete.");
    Ok(())
}

fn install_software(software_list: Vec<String>, rootfs: &str) -> Result<(), Box<dyn std::error::Error>> {
    for software in software_list {
        match software.as_str() {
            "node" => {
                println!("Installing Node.js...");
                install_nodejs(None, rootfs)?;
            },
            "python3" => {
                println!("Installing Python 3...");
                install_python(None, rootfs)?;
            },
            "rust" => {
                println!("Installing Rust...");
                install_rust(None, rootfs)?;
            },
            other => eprintln!("Unknown software: {}", other),
        }
    }
    Ok(())
}

fn install_python(version: Option<&str>, rootfs: &str) -> Result<(), Box<dyn std::error::Error>> {
    let python_version = version.unwrap_or("latest");
    let python_url = if python_version == "latest" {
        "https://www.python.org/ftp/python/3.10.6/Python-3.10.6.tgz".to_string()
    } else {
        format!("https://www.python.org/ftp/python/{}/Python-{}.tgz", python_version, python_version)
    };

    let python_dir = Path::new(rootfs).join("tmp/python");
    fs::create_dir_all(&python_dir)?;

    let tarball_path = python_dir.join("python.tgz");
    println!("Downloading Python from {}...", python_url);
    download_file(&python_url, &tarball_path)?;

    println!("Extracting Python tarball into {}...", python_dir.display());
    let file = fs::File::open(&tarball_path)?;
    let tar = GzDecoder::new(file);
    let mut archive = Archive::new(tar);
    archive.unpack(&python_dir)?;
    println!("Python installation complete.");

    if let Err(e) = create_symlinks(python_dir.to_str().unwrap(), "Python*/bin") {
        println!("Warning: {}", e);
    }
    Ok(())
}

fn install_rust(version: Option<&str>, rootfs: &str) -> Result<(), Box<dyn std::error::Error>> {
    let rust_version = version.unwrap_or("stable");
    let rustup_script_url = if rust_version == "stable" {
        "https://sh.rustup.rs".to_string()
    } else {
        format!("https://rust-lang.github.io/rustup/dist/{}/rustup-init", rust_version)
    };

    let rust_dir = Path::new(rootfs).join("tmp/rust");
    fs::create_dir_all(&rust_dir)?;

    let rustup_script_path = rust_dir.join("rustup-init.sh");
    println!("Downloading rustup script from {}...", rustup_script_url);
    download_file(&rustup_script_url, &rustup_script_path)?;

    println!("Running rustup installation script in {}...", rust_dir.display());
    Command::new("sh")
        .arg(rustup_script_path)
        .arg("-y")
        .current_dir(rust_dir.clone())
        .output()?;

    println!("Rust installation complete.");
    create_symlinks(rust_dir.to_str().unwrap(), "cargo/bin")?;
    Ok(())
}

fn install_nodejs(version: Option<&str>, rootfs: &str) -> Result<(), Box<dyn std::error::Error>> {
    let node_version = version.unwrap_or("latest");
    let node_url = if node_version == "latest" {
        "https://nodejs.org/dist/v23.7.0/node-v23.7.0-linux-x64.tar.xz".to_string()
    } else {
        format!("https://nodejs.org/dist/v{}/node-v{}-linux-x64.tar.xz", node_version, node_version)
    };

    let node_dir = Path::new(rootfs).join("tmp/node");
    fs::create_dir_all(&node_dir)?;

    let tarball_path = node_dir.join("node.tar.xz");
    println!("Downloading Node.js from {}...", node_url);
    download_file(&node_url, &tarball_path)?;

    println!("Extracting Node.js tarball into {}...", node_dir.display());
    let file = fs::File::open(&tarball_path)?;
    let tar = XzDecoder::new(file);
    let mut archive = Archive::new(tar);
    archive.unpack(&node_dir)?;
    println!("Node.js installation complete.");

    create_symlinks(node_dir.to_str().unwrap(), "node-v*/bin")?;
    Ok(())
}

fn create_symlinks(source_dir: &str, bin_pattern: &str) -> Result<(), Box<dyn std::error::Error>> {
    let search_pattern = format!("{}/{}", source_dir, bin_pattern);
    println!("Creating symlinks using glob pattern: {}", search_pattern);

    let mut found = false;
    for entry in glob(&search_pattern)? {
        match entry {
            Ok(path) => {
                if path.is_dir() {
                    println!("Found directory: {}", path.display());
                    for file_entry in fs::read_dir(&path)? {
                        let file_entry = file_entry?;
                        let file_path = file_entry.path();
                        if file_path.is_file() {
                            let dest = Path::new("/usr/local/bin").join(file_path.file_name().unwrap());
                            println!("Linking {} -> {}", file_path.display(), dest.display());
                            if dest.exists() {
                                println!("Destination {} exists, removing...", dest.display());
                                fs::remove_file(&dest)?;
                            }
                            std::os::unix::fs::symlink(&file_path, &dest)?;
                        }
                    }
                    found = true;
                }
            },
            Err(e) => eprintln!("Glob error: {:?}", e),
        }
    }
    if !found {
        println!("Warning: No directories matched the glob pattern: {}", search_pattern);
    }
    Ok(())
}