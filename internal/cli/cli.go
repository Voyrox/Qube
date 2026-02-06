package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Voyrox/Qube/internal/core/container"
	"github.com/Voyrox/Qube/internal/core/tracking"
	"github.com/fatih/color"
	"gopkg.in/yaml.v3"
)

var (
	runContainerFn       = container.RunContainer
	validateImageFn      = container.ValidateImage
	convertAndRunFn      = container.ConvertAndRun
	pullImageFromHubFn   = container.PullImageFromHub
	getAllTrackedEntries = tracking.GetAllTrackedEntries
	getProcessUptimeFn   = tracking.GetProcessUptime
	stopContainerFn      = container.StopContainer
	deleteContainerFn    = container.DeleteContainer
)

type QMLConfig struct {
	Container struct {
		System      string              `yaml:"system"`
		Ports       []string            `yaml:"ports"`
		Cmd         []string            `yaml:"cmd"`
		Isolated    bool                `yaml:"isolated"`
		Environment map[string]string   `yaml:"environment"`
		Volumes     []map[string]string `yaml:"volumes"`
		Debug       bool                `yaml:"debug"`
	} `yaml:"container"`
}

func RunCommand(args []string) {
	if _, err := os.Stat("qube.yml"); err == nil && len(args) == 2 {
		runFromQML()
		return
	}

	var image string
	var ports string
	var isolated bool
	var volumes [][2]string
	var envVars []string

	cmdFlagIndex := -1
	for i, arg := range args {
		if arg == "--cmd" {
			cmdFlagIndex = i
			break
		}
	}

	if cmdFlagIndex == -1 {
		color.Red("Usage: qube run [--image <image>] [--ports <ports>] [--env <NAME=VALUE>] [--volume /host/path:/container/path] [--isolated] [--debug] --cmd \"<command>\"")
		os.Exit(1)
	}

	for i := 2; i < cmdFlagIndex; i++ {
		switch args[i] {
		case "--image":
			if i+1 < cmdFlagIndex {
				image = args[i+1]
				i++
			}
		case "--ports":
			if i+1 < cmdFlagIndex {
				ports = args[i+1]
				i++
			}
		case "--isolated":
			isolated = true
		case "--volume":
			if i+1 < cmdFlagIndex {
				parts := strings.SplitN(args[i+1], ":", 2)
				if len(parts) != 2 {
					color.Red("Error: --volume argument must be in the format /host/path:/container/path")
					os.Exit(1)
				}
				volumes = append(volumes, [2]string{parts[0], parts[1]})
				i++
			}
		case "--env":
			if i+1 < cmdFlagIndex {
				if strings.Contains(args[i+1], "=") {
					envVars = append(envVars, args[i+1])
					i++
				} else {
					color.Red("Error: --env argument must be in the format KEY=VALUE")
					os.Exit(1)
				}
			}
		}
	}

	if image == "" {
		color.Red("Error: --image flag must be specified.")
		os.Exit(1)
	}

	if err := container.ValidateImage(image); err != nil {
		color.Red("Error: Invalid image provided ('%s'). Reason: %v", image, err)
		os.Exit(1)
	}

	userCmd := args[cmdFlagIndex+1:]
	cwd, err := os.Getwd()
	if err != nil {
		color.Red("Failed to get current directory: %v", err)
		os.Exit(1)
	}

	if err := container.RunContainer("", cwd, userCmd, false, image, ports, isolated, volumes, envVars); err != nil {
		color.Red("Failed to run container: %v", err)
		os.Exit(1)
	}
}

func runFromQML() {
	data, err := os.ReadFile("qube.yml")
	if err != nil {
		color.Red("Failed to read qube.yml: %v", err)
		os.Exit(1)
	}

	var config QMLConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		color.Red("Failed to parse qube.yml: %v", err)
		os.Exit(1)
	}

	var volumes [][2]string
	for _, vol := range config.Container.Volumes {
		hostPath := vol["host_path"]
		containerPath := vol["container_path"]
		if hostPath != "" && containerPath != "" {
			volumes = append(volumes, [2]string{hostPath, containerPath})
		}
	}

	var envVars []string
	for k, v := range config.Container.Environment {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	ports := strings.Join(config.Container.Ports, ",")
	cwd, _ := os.Getwd()

	cmdStr := strings.Join(config.Container.Cmd, " && ")
	cmdArray := []string{cmdStr}

	if err := container.RunContainer("", cwd, cmdArray, config.Container.Debug, config.Container.System, ports, config.Container.Isolated, volumes, envVars); err != nil {
		color.Red("Failed to run container: %v", err)
		os.Exit(1)
	}
}

func ListCommand() {
	container.ListContainers()
}

func StopCommand(args []string) {
	if len(args) < 3 {
		color.Red("Usage: qube stop <pid|container_name>")
		os.Exit(1)
	}

	nameOrPID := args[2]
	pid, err := strconv.Atoi(nameOrPID)

	if err == nil {
		if err := container.StopContainer(pid); err != nil {
			color.Red("Failed to stop container: %v", err)
			os.Exit(1)
		}
	} else {
		entries := tracking.GetAllTrackedEntries()
		found := false
		for _, entry := range entries {
			if entry.Name == nameOrPID {
				if err := container.StopContainer(entry.PID); err != nil {
					color.Red("Failed to stop container: %v", err)
					os.Exit(1)
				}
				found = true
				break
			}
		}
		if !found {
			color.Red("Container %s not found", nameOrPID)
			os.Exit(1)
		}
	}
}

func StartCommand(args []string) {
	if len(args) < 3 {
		color.Red("Usage: qube start <pid|container_name>")
		os.Exit(1)
	}

	nameOrPID := args[2]
	if err := container.StartContainer(nameOrPID); err != nil {
		color.Red("Failed to start container: %v", err)
		os.Exit(1)
	}
}

func DeleteCommand(args []string) {
	if len(args) < 3 {
		color.Red("Usage: qube delete <pid|container_name>")
		os.Exit(1)
	}

	nameOrPID := args[2]
	if err := container.DeleteContainer(nameOrPID); err != nil {
		color.Red("Failed to delete container: %v", err)
		os.Exit(1)
	}
}

func EvalCommand(args []string) {
	if len(args) < 3 {
		color.Red("Usage: qube eval <container_name|pid> [command]")
		os.Exit(1)
	}

	nameOrPID := args[2]
	entries := tracking.GetAllTrackedEntries()

	var targetEntry *tracking.ContainerEntry
	for _, entry := range entries {
		if entry.Name == nameOrPID || fmt.Sprintf("%d", entry.PID) == nameOrPID {
			targetEntry = &entry
			break
		}
	}

	if targetEntry == nil {
		color.Red("Container %s not found", nameOrPID)
		os.Exit(1)
	}

	rootfs := container.GetRootfs(targetEntry.Name)

	cmd := "sh"
	if len(args) > 3 {
		cmd = strings.Join(args[3:], " ")
	}

	execCmd := exec.Command("nsenter", "-t", fmt.Sprintf("%d", targetEntry.PID), "-m", "-u", "-i", "-p", "chroot", rootfs, "/bin/sh", "-c", cmd)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Run()
}

func InfoCommand(args []string) {
	if len(args) < 3 {
		color.Red("Usage: qube info <container_name|pid>")
		os.Exit(1)
	}

	nameOrPID := args[2]
	entries := tracking.GetAllTrackedEntries()

	for _, entry := range entries {
		if entry.Name == nameOrPID || fmt.Sprintf("%d", entry.PID) == nameOrPID {
			fmt.Println()
			color.New(color.Bold, color.FgCyan).Printf("Container: %s\n", entry.Name)
			fmt.Printf("  PID: %d\n", entry.PID)
			fmt.Printf("  Image: %s\n", entry.Image)
			fmt.Printf("  Working Directory: %s\n", entry.Dir)
			fmt.Printf("  Command: %v\n", entry.Command)
			fmt.Printf("  Timestamp: %d\n", entry.Timestamp)

			if entry.Ports != "" {
				fmt.Printf("  Ports: %s\n", entry.Ports)
			}

			fmt.Printf("  Isolated: %t\n", entry.Isolated)

			if entry.PID > 0 {
				if uptime, err := tracking.GetProcessUptime(entry.PID); err == nil {
					fmt.Printf("  Uptime: %d seconds\n", uptime)
				}
			}

			fmt.Println()
			return
		}
	}

	color.Red("Container %s not found", nameOrPID)
	os.Exit(1)
}

func SnapshotCommand(args []string) {
	if len(args) < 3 {
		color.Red("Usage: qube snapshot <container_name|pid>")
		os.Exit(1)
	}

	nameOrPID := args[2]
	entries := tracking.GetAllTrackedEntries()

	for _, entry := range entries {
		if entry.Name == nameOrPID || fmt.Sprintf("%d", entry.PID) == nameOrPID {
			snapshotPath := filepath.Join(entry.Dir, fmt.Sprintf("snapshot_%d.tar.gz", time.Now().Unix()))

			color.Blue("Creating snapshot of %s...", entry.Name)

			rootfs := container.GetRootfs(entry.Name)
			cmd := exec.Command("tar", "-czf", snapshotPath, "-C", filepath.Dir(rootfs), "rootfs")
			if err := cmd.Run(); err != nil {
				color.Red("Failed to create snapshot: %v", err)
				os.Exit(1)
			}

			color.Green("✓ Snapshot created: %s", snapshotPath)
			return
		}
	}

	color.Red("Container %s not found", nameOrPID)
	os.Exit(1)
}

func DockerCommand(args []string) {
	if len(args) < 3 {
		color.Red("Usage: qube docker <Dockerfile_path>")
		os.Exit(1)
	}

	dockerfilePath := args[2]

	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		color.Red("Dockerfile not found: %s", dockerfilePath)
		os.Exit(1)
	}

	if err := container.ConvertAndRun(dockerfilePath); err != nil {
		color.Red("Failed to convert Dockerfile: %v", err)
		os.Exit(1)
	}
}

func PullCommand(args []string) {
	if len(args) < 3 {
		color.Red("Usage: qube pull <user>:<image>:<version>")
		color.Yellow("Example: qube pull Voyrox:nodejs:1.1.0")
		os.Exit(1)
	}

	imageSpec := args[2]
	parts := strings.Split(imageSpec, ":")

	if len(parts) != 3 {
		color.Red("Error: Image must be in format <user>:<image>:<version>")
		color.Yellow("Example: Voyrox:nodejs:1.1.0")
		os.Exit(1)
	}

	user := parts[0]
	image := parts[1]
	version := parts[2]

	color.Blue("Pulling image: %s/%s version %s from Qube Hub...", user, image, version)

	if err := container.PullImageFromHub(user, image, version); err != nil {
		color.Red("Failed to pull image: %v", err)
		os.Exit(1)
	}

	color.Green("✓ Successfully pulled %s:%s:%s", user, image, version)
}
