package container

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Voyrox/Qube/internal/config"
	"golang.org/x/sys/unix"
)

func GetRootfs(cid string) string {
	return filepath.Join(config.QubeContainersBase, cid, "rootfs")
}

func PrepareRootfsDir(cid string) error {
	rootfs := GetRootfs(cid)
	if _, err := os.Stat(rootfs); err == nil {
		if err := os.RemoveAll(rootfs); err != nil {
			return err
		}
	}
	return os.MkdirAll(rootfs, 0755)
}

func CopyDirectoryIntoHome(cid, workDir string) error {
	rootfs := GetRootfs(cid)
	workspacePath := filepath.Join(rootfs, "workspace")

	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return err
	}

	rsyncCmd := exec.Command("rsync", "-a", "--exclude=.git", workDir+"/", workspacePath+"/")
	if err := rsyncCmd.Run(); err == nil {
		return nil
	}

	cpCmd := exec.Command("sh", "-c", fmt.Sprintf("cp -rT %s %s", workDir, workspacePath))
	if err := cpCmd.Run(); err != nil {
		return fmt.Errorf("failed to copy %s -> %s: %w", workDir, workspacePath, err)
	}

	return nil
}

func MountProc(cid string) error {
	rootfs := GetRootfs(cid)
	procPath := filepath.Join(rootfs, "proc")

	if err := os.MkdirAll(procPath, 0755); err != nil {
		return err
	}

	return unix.Mount("proc", procPath, "proc", unix.MS_NOEXEC|unix.MS_NOSUID|unix.MS_NODEV, "")
}

func MountVolume(cid, hostPath, containerPath string) error {
	if _, err := os.Stat(hostPath); os.IsNotExist(err) {
		return fmt.Errorf("host path '%s' does not exist", hostPath)
	}

	rootfs := GetRootfs(cid)
	dest := filepath.Join(rootfs, strings.TrimPrefix(containerPath, "/"))

	if _, err := os.Stat(dest); os.IsNotExist(err) {
		fmt.Printf("DEBUG: Creating mount point at %s\n", dest)
		if err := os.MkdirAll(dest, 0755); err != nil {
			return err
		}
	}

	fmt.Printf("DEBUG: Attempting to mount %s -> %s\n", hostPath, dest)

	unix.Unmount(dest, unix.MNT_DETACH)

	return unix.Mount(hostPath, dest, "", unix.MS_BIND, "")
}

func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func CopyDir(src, dst string) error {
	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
