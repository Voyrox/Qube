package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Voyrox/Qube/internal/core/cgroup"
	"github.com/Voyrox/Qube/internal/core/tracking"
	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/sys/unix"
)

func RunContainer(existingName, workDir string, userCmd []string, debug bool, image, ports string, isolated bool, volumes [][2]string, envVars []string) error {
	if len(userCmd) == 0 {
		return fmt.Errorf("no command specified to launch in container")
	}

	containerID := existingName
	if containerID == "" {
		containerID = GenerateContainerID()
	}

	rootfs := GetRootfs(containerID)
	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		bar := progressbar.NewOptions(4,
			progressbar.OptionSetDescription("Launching container"),
		)

		bar.Describe("Preparing container filesystem...")
		if err := PrepareRootfsDir(containerID); err != nil {
			return err
		}
		bar.Add(1)

		bar.Describe("Extracting container image...")
		if err := ExtractRootfsTar(containerID, image); err != nil {
			bar.Finish()
			color.Red("Error: Invalid image provided ('%s'). Reason: %v", image, err)
			tracking.RemoveContainerFromTrackingByName(containerID)
			return err
		}
		bar.Add(1)

		bar.Describe("Copying working directory...")
		if err := CopyDirectoryIntoHome(containerID, workDir); err != nil {
			return err
		}
		bar.Add(1)

		bar.Describe("Launching container...")
		bar.Add(1)
		bar.Finish()
		color.Green("Container build complete!")
	} else {
		fmt.Println("Container filesystem already exists. Skipping build.")
	}

	cgroupPath, err := cgroup.SetupCgroupForContainer(containerID)
	if err != nil {
		fmt.Printf("Warning: Failed to setup cgroup: %v. Container will run without resource limits.\n", err)
		cgroupPath = ""
	} else if debug {
		fmt.Printf("Cgroup created at: %s\n", cgroupPath)
	}

	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}

	cmd := exec.Command("/proc/self/exe", "__container_init__")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("QUBE_CONTAINER_ID=%s", containerID),
		fmt.Sprintf("QUBE_ROOTFS=%s", rootfs),
		fmt.Sprintf("QUBE_CMD=%s", userCmd[0]),
		fmt.Sprintf("QUBE_ISOLATED=%t", isolated),
		fmt.Sprintf("QUBE_PIPE_FD=%d", w.Fd()),
	)

	for i, arg := range userCmd {
		cmd.Env = append(cmd.Env, fmt.Sprintf("QUBE_ARG_%d=%s", i, arg))
	}

	for i, env := range envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("QUBE_ENV_%d=%s", i, env))
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWIPC | syscall.CLONE_NEWUTS,
	}

	if isolated {
		cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWNET
	}

	cmd.ExtraFiles = []*os.File{w}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	w.Close()

	pidBuf := make([]byte, 10)
	n, err := r.Read(pidBuf)
	r.Close()

	if err != nil || n == 0 {
		return fmt.Errorf("container process did not report a final PID")
	}

	childPID := cmd.Process.Pid

	if cgroupPath != "" {
		if err := cgroup.AddProcessToCgroup(cgroupPath, childPID); err != nil {
			fmt.Printf("Warning: Failed to add process %d to cgroup: %v\n", childPID, err)
		} else if debug {
			fmt.Printf("Process %d added to cgroup\n", childPID)
		}
	}

	if existingName != "" {
		tracking.UpdateContainerPID(containerID, childPID, workDir, userCmd, image, ports, isolated, volumes, envVars)
	} else {
		tracking.TrackContainerNamed(containerID, childPID, workDir, userCmd, image, ports, isolated, volumes, envVars)
	}

	color.Green("✓ Container %s started with PID %d", containerID, childPID)
	fmt.Printf("  Image: %s\n", image)
	fmt.Printf("  Working directory: %s\n", workDir)
	fmt.Printf("  Command: %v\n", userCmd)
	if ports != "" {
		fmt.Printf("  Ports: %s\n", ports)
	}
	if isolated {
		fmt.Println("  Network: Isolated")
	}

	return nil
}

func ContainerInit() error {
	_ = os.Getenv("QUBE_CONTAINER_ID")
	rootfs := os.Getenv("QUBE_ROOTFS")
	pipeFdStr := os.Getenv("QUBE_PIPE_FD")

	if pipeFdStr != "" {
		pidStr := fmt.Sprintf("%d", os.Getpid())
		if fd := os.NewFile(3, "pipe"); fd != nil {
			fd.Write([]byte(pidStr))
			fd.Close()
		}
	}

	if err := unix.Mount("proc", filepath.Join(rootfs, "proc"), "proc", unix.MS_NOEXEC|unix.MS_NOSUID|unix.MS_NODEV, ""); err != nil {
		return fmt.Errorf("failed to mount proc: %w", err)
	}

	if err := syscall.Chroot(rootfs); err != nil {
		return fmt.Errorf("failed to chroot: %w", err)
	}

	if err := os.Chdir("/workspace"); err != nil {
		if err := os.Chdir("/"); err != nil {
			return fmt.Errorf("failed to change directory: %w", err)
		}
	}

	i := 0
	for {
		env := os.Getenv(fmt.Sprintf("QUBE_ENV_%d", i))
		if env == "" {
			break
		}
		os.Setenv(env[:strings.Index(env, "=")], env[strings.Index(env, "=")+1:])
		i++
	}

	var args []string
	i = 0
	for {
		arg := os.Getenv(fmt.Sprintf("QUBE_ARG_%d", i))
		if arg == "" {
			break
		}
		args = append(args, arg)
		i++
	}

	if len(args) == 0 {
		return fmt.Errorf("no command to execute")
	}

	return syscall.Exec("/bin/sh", append([]string{"/bin/sh", "-c"}, args...), os.Environ())
}

func StartContainer(nameOrPID string) error {
	entries := tracking.GetAllTrackedEntries()

	for _, entry := range entries {
		if entry.Name == nameOrPID || fmt.Sprintf("%d", entry.PID) == nameOrPID {
			if entry.PID > 0 {
				procPath := fmt.Sprintf("/proc/%d", entry.PID)
				if _, err := os.Stat(procPath); err == nil {
					color.Yellow("Container %s is already running with PID %d", entry.Name, entry.PID)
					return nil
				}
			}

			return RunContainer(entry.Name, entry.Dir, entry.Command, false, entry.Image, entry.Ports, entry.Isolated, entry.Volumes, entry.EnvVars)
		}
	}

	return fmt.Errorf("container %s not found", nameOrPID)
}

func EvalCommand(pid int, command string) (string, error) {
	cmd := exec.Command("nsenter", "-t", fmt.Sprintf("%d", pid), "-m", "-u", "-i", "-n", "-p", "--", "/bin/sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %v", err)
	}
	return string(output), nil
}
