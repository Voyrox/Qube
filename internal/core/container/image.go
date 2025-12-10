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
	imageFilename := fmt.Sprintf("%s.tar", image)
	imagePath := filepath.Join(imagesDir, imageFilename)

	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		if err := os.MkdirAll(imagesDir, 0755); err != nil {
			return "", err
		}

		color.Blue("Image %s not found locally. Downloading...", image)

		url := fmt.Sprintf("%s/files/%s.tar", config.BaseURL, image)
		resp, err := http.Get(url)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("failed to download image from %s. Status: %s", url, resp.Status)
		}

		file, err := os.Create(imagePath)
		if err != nil {
			return "", err
		}
		defer file.Close()

		bar := progressbar.DefaultBytes(
			resp.ContentLength,
			"Downloading image",
		)

		_, err = io.Copy(io.MultiWriter(file, bar), resp.Body)
		if err != nil {
			return "", err
		}

		color.Green("Download complete")
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

	cmd := exec.Command("tar", "-xf", imagePath, "-C", rootfs)
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
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".tar") {
			imageName := strings.TrimSuffix(file.Name(), ".tar")
			images = append(images, imageName)
		}
	}

	return images, nil
}
