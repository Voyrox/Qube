package container

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Voyrox/Qube/internal/config"
	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

func EnsureImageExists(image string) (string, error) {
	imagesDir := filepath.Join(config.QubeContainersBase, "images")

	if strings.Count(image, ":") != 2 {
		return "", fmt.Errorf("image must be in format <user>:<image>:<version>. Example: Voyrox:nodejs:1.1.0")
	}

	parts := strings.Split(image, ":")
	user, imgName, version := parts[0], parts[1], parts[2]

	imageFilename := fmt.Sprintf("%s_%s_%s.tar.gz", user, imgName, version)
	imagePath := filepath.Join(imagesDir, imageFilename)

	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		color.Blue("Image %s not found locally. Pulling from Qube Hub...", image)
		if err := PullImageFromHub(user, imgName, version); err != nil {
			return "", err
		}
	}

	return imagePath, nil
}

func ValidateImage(image string) error {
	_, err := EnsureImageExists(image)
	return err
}

func ExtractRootfsTar(cid, image string) error {
	rootfs := GetRootfs(cid)
	imagePath, err := EnsureImageExists(image)
	if err != nil {
		return err
	}

	cmd := exec.Command("tar", "--numeric-owner", "-xzf", imagePath, "-C", rootfs)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract the image %s: %w", imagePath, err)
	}

	return nil
}

func ListImages() ([]string, error) {
	imagesDir := filepath.Join(config.QubeContainersBase, "images")

	if _, err := os.Stat(imagesDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	files, err := ioutil.ReadDir(imagesDir)
	if err != nil {
		return nil, err
	}

	var images []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".tar.gz") {
			imageName := strings.TrimSuffix(file.Name(), ".tar.gz")
			images = append(images, imageName)
		}
	}

	return images, nil
}

func PullImageFromHub(user, image, version string) error {
	imagesDir := filepath.Join(config.QubeContainersBase, "images")

	imageFilename := fmt.Sprintf("%s_%s_%s.tar.gz", user, image, version)
	imagePath := filepath.Join(imagesDir, imageFilename)

	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return fmt.Errorf("failed to create images directory: %w", err)
	}

	if _, err := os.Stat(imagePath); err == nil {
		color.Yellow("Image already exists locally: %s", imageFilename)
		return nil
	}

	url := fmt.Sprintf("%s/download/%s/%s?version=%s", config.BaseURL, user, image, version)

	color.Blue("Downloading from: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to download image from %s. Status: %s, Response: %s", url, resp.Status, string(body))
	}

	file, err := os.Create(imagePath)
	if err != nil {
		return fmt.Errorf("failed to create image file: %w", err)
	}
	defer file.Close()

	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		"Downloading image",
	)

	_, err = io.Copy(io.MultiWriter(file, bar), resp.Body)
	if err != nil {
		os.Remove(imagePath)
		return fmt.Errorf("failed to save image: %w", err)
	}

	color.Green("✓ Download complete: %s", imageFilename)
	return nil
}
