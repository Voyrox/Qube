use colored::Colorize;
use indicatif::{ProgressBar, ProgressStyle};
use reqwest;
use std::fs::{self, File};
use std::io::{Read, Write};
use std::path::Path;
use std::process::Command;

use crate::config::QUBE_CONTAINERS_BASE;
use crate::config::BASE_URL;

pub fn ensure_image_exists(image: &str) -> Result<String, Box<dyn std::error::Error>> {
    let images_dir = format!("{}/images", QUBE_CONTAINERS_BASE);
    let image_path = format!("{}/{}", images_dir, image);
    if !Path::new(&image_path).exists() {
        fs::create_dir_all(&images_dir)?;
        println!("{}", format!("Image {} not found locally. Downloading...", image).blue());

        let url = format!("{}/files/{}", BASE_URL, image);
        let mut resp = reqwest::blocking::get(&url)?;
        if !resp.status().is_success() {
            return Err(format!("Failed to download image from {}. Status: {}", url, resp.status()).into());
        }
        let total_size = resp.content_length().unwrap_or(0);
        let pb = ProgressBar::new(total_size);
        pb.set_style(
            ProgressStyle::default_bar()
                .template("{spinner:.green} [{elapsed_precise}] [{bar:40.cyan/red}] {bytes}/{total_bytes} ({eta}, {percent}%) Downloading image")
                .unwrap()
                .progress_chars("∷∷"),
        );
        let mut file = File::create(&image_path)?;
        let mut buffer = [0; 8192];
        let mut downloaded: u64 = 0;
        loop {
            let n = resp.read(&mut buffer)?;
            if n == 0 {
                break;
            }
            file.write_all(&buffer[..n])?;
            downloaded += n as u64;
            pb.set_position(downloaded);
        }
        pb.finish_with_message("Download complete");
    }
    Ok(image_path)
}

pub fn validate_image(image: &str) -> Result<(), Box<dyn std::error::Error>> {
    ensure_image_exists(image)?;
    Ok(())
}

pub fn extract_rootfs_tar(cid: &str, image: &str) -> Result<(), Box<dyn std::error::Error>> {
    let rootfs = crate::container::fs::get_rootfs(cid);
    let image_path = ensure_image_exists(image)?;
    
    let status = Command::new("tar")
        .args(["-xf", &image_path, "-C", &rootfs])
        .status()?;
    if !status.success() {
        return Err(format!("Failed to extract the image {}!", image_path).into());
    }
    Ok(())
}
