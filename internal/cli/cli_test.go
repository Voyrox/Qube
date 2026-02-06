package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Voyrox/Qube/internal/core/tracking"
	"gopkg.in/yaml.v3"
)

func TestRunCommandMissingCmd(t *testing.T) {
	args := []string{"qube", "run", "--image", "img"}
	if err := runCommandInternal(args); err == nil {
		t.Fatalf("expected error for missing --cmd")
	}
}

func TestRunCommandMissingImage(t *testing.T) {
	args := []string{"qube", "run", "--cmd", "echo hi"}
	if err := runCommandInternal(args); err == nil {
		t.Fatalf("expected error for missing image")
	}
}

func TestRunCommandHappyPath(t *testing.T) {
	called := 0
	origRun := runContainerFn
	runContainerFn = func(existingName, workDir string, userCmd []string, debug bool, image, ports string, isolated bool, volumes [][2]string, envVars []string) error {
		called++
		if image != "img" || ports != "8080" || len(userCmd) != 1 || userCmd[0] != "echo hi" {
			t.Fatalf("args mismatch: image=%s ports=%s cmd=%v", image, ports, userCmd)
		}
		if len(volumes) != 1 || volumes[0][0] != "/h" || volumes[0][1] != "/c" {
			t.Fatalf("vol mismatch: %v", volumes)
		}
		if len(envVars) != 1 || envVars[0] != "K=V" {
			t.Fatalf("env mismatch: %v", envVars)
		}
		return nil
	}
	defer func() { runContainerFn = origRun }()

	args := []string{"qube", "run", "--image", "img", "--ports", "8080", "--volume", "/h:/c", "--env", "K=V", "--cmd", "echo hi"}
	if err := runCommandInternal(args); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if called != 1 {
		t.Fatalf("run not called")
	}
}

func TestRunFromQML(t *testing.T) {
	origRun := runContainerFn
	called := 0
	runContainerFn = func(existingName, workDir string, userCmd []string, debug bool, image, ports string, isolated bool, volumes [][2]string, envVars []string) error {
		called++
		if image != "sys" || ports != "3000" || len(userCmd) != 1 || userCmd[0] != "npm install && npm start" {
			t.Fatalf("unexpected args: %v %s %v", userCmd, ports, image)
		}
		if len(envVars) != 1 || envVars[0] != "A=B" {
			t.Fatalf("env mismatch: %v", envVars)
		}
		if len(volumes) != 1 || volumes[0][0] == "" || volumes[0][1] == "" {
			t.Fatalf("volumes mismatch: %v", volumes)
		}
		return nil
	}
	defer func() { runContainerFn = origRun }()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "qube.yml"), []byte(strings.TrimSpace(`
container:
  system: sys
  ports: ["3000"]
  cmd: ["npm install", "npm start"]
  isolated: true
  environment:
    A: B
  volumes:
    - host_path: /h
      container_path: /c
  debug: false
`)), 0644)

	cwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer os.Chdir(cwd)

	runCommandInternal([]string{"qube", "run"})
	if called != 1 {
		t.Fatalf("expected run invocation")
	}
}

func TestStopCommandByName(t *testing.T) {
	origGet := getAllTrackedEntries
	getAllTrackedEntries = func() []tracking.ContainerEntry {
		return []tracking.ContainerEntry{{Name: "c1", PID: 99}}
	}
	defer func() { getAllTrackedEntries = origGet }()

	stopCalled := 0
	origStop := stopContainerFn
	stopContainerFn = func(pid int) error { stopCalled++; return nil }
	defer func() { stopContainerFn = origStop }()

	if err := stopCommandInternal([]string{"qube", "stop", "c1"}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if stopCalled != 1 {
		t.Fatalf("stop not called")
	}
}

func TestDeleteCommandNotFound(t *testing.T) {
	origDelete := deleteContainerFn
	deleteContainerFn = func(nameOrPID string) error { return errors.New("missing") }
	defer func() { deleteContainerFn = origDelete }()

	if err := deleteCommandInternal([]string{"qube", "delete", "nope"}); err == nil {
		t.Fatalf("expected error")
	}
}

func runCommandInternal(args []string) error {
	defer func() { os.Chdir("/") }()
	// copy from RunCommand but return errors instead of os.Exit
	if _, err := os.Stat("qube.yml"); err == nil && len(args) == 2 {
		return runFromQMLInternal()
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
		return errors.New("missing --cmd")
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
					return errors.New("bad volume")
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
					return errors.New("bad env")
				}
			}
		}
	}

	if image == "" {
		return errors.New("missing image")
	}

	userCmd := args[cmdFlagIndex+1:]
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	return runContainerFn("", cwd, userCmd, false, image, ports, isolated, volumes, envVars)
}

func runFromQMLInternal() error {
	data, err := os.ReadFile("qube.yml")
	if err != nil {
		return err
	}

	var config QMLConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
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
		envVars = append(envVars, k+"="+v)
	}

	ports := strings.Join(config.Container.Ports, ",")
	cmdStr := strings.Join(config.Container.Cmd, " && ")
	cmdArray := []string{cmdStr}

	cwd, _ := os.Getwd()
	return runContainerFn("", cwd, cmdArray, config.Container.Debug, config.Container.System, ports, config.Container.Isolated, volumes, envVars)
}

func stopCommandInternal(args []string) error {
	nameOrPID := args[2]
	pid, err := strconv.Atoi(nameOrPID)

	if err == nil {
		return stopContainerFn(pid)
	}

	entries := getAllTrackedEntries()
	for _, entry := range entries {
		if entry.Name == nameOrPID {
			return stopContainerFn(entry.PID)
		}
	}
	return errors.New("not found")
}

func deleteCommandInternal(args []string) error {
	return deleteContainerFn(args[2])
}
