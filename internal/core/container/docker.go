package container

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Voyrox/Qube/internal/core/tracking"
	"github.com/fatih/color"
	"gopkg.in/yaml.v3"
)

type DockerfileConfig struct {
	From        string
	Expose      []string
	Cmd         []string
	Env         map[string]string
	InstallCmds []string
}

type QubeConfig struct {
	System      string         `yaml:"system"`
	Cmd         interface{}    `yaml:"cmd"`
	Ports       []string       `yaml:"ports,omitempty"`
	Environment interface{}    `yaml:"enviroment,omitempty"`
	Isolated    bool           `yaml:"isolated,omitempty"`
	Volumes     []VolumeConfig `yaml:"volumes,omitempty"`
}

type VolumeConfig struct {
	HostPath      string `yaml:"host_path"`
	ContainerPath string `yaml:"container_path"`
}

func ConvertAndRun(dockerfilePath string) error {
	config, err := parseDockerfile(dockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to parse Dockerfile: %w", err)
	}

	qubeConfig := convertToQubeYaml(config)

	dockerfileDir := filepath.Dir(dockerfilePath)
	qubeYamlPath := filepath.Join(dockerfileDir, "qube.yml")

	yamlData, err := yaml.Marshal(qubeConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	if err := os.WriteFile(qubeYamlPath, yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write qube.yml: %w", err)
	}

	color.Green("Converted Dockerfile to %s", qubeYamlPath)

	return buildFromQubeYaml(qubeYamlPath, dockerfileDir, qubeConfig)
}

func parseDockerfile(path string) (*DockerfileConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := &DockerfileConfig{
		Env:    make(map[string]string),
		Expose: []string{},
		Cmd:    []string{},
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "FROM ") {
			config.From = strings.TrimSpace(strings.TrimPrefix(line, "FROM "))
		}

		if strings.HasPrefix(line, "EXPOSE ") {
			ports := strings.Fields(strings.TrimPrefix(line, "EXPOSE "))
			config.Expose = append(config.Expose, ports...)
		}

		if strings.HasPrefix(line, "CMD ") {
			cmdStr := strings.TrimSpace(strings.TrimPrefix(line, "CMD "))

			if strings.HasPrefix(cmdStr, "[") {
				var cmdArray []string
				if err := json.Unmarshal([]byte(cmdStr), &cmdArray); err == nil {
					config.Cmd = cmdArray
				} else {
					config.Cmd = []string{cmdStr}
				}
			} else {
				config.Cmd = []string{cmdStr}
			}
		}

		if strings.HasPrefix(line, "ENV ") {
			envStr := strings.TrimSpace(strings.TrimPrefix(line, "ENV "))
			parts := strings.SplitN(envStr, " ", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := strings.Trim(parts[1], "\"'")
				config.Env[key] = value
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	config.InstallCmds = generateInstallCommands(config.Env)

	return config, nil
}

func generateInstallCommands(env map[string]string) []string {
	var commands []string

	if nodeVersion, ok := env["INSTALL_NODE"]; ok && nodeVersion != "" {
		commands = append(commands,
			"curl -fsSL https://deb.nodesource.com/setup_lts.x | bash -",
			"apt-get install -y nodejs",
		)
		if nodeVersion != "latest" {
			commands = append(commands, fmt.Sprintf("npm install -g n && n %s", nodeVersion))
		}
	}

	if rustVersion, ok := env["INSTALL_RUST"]; ok && rustVersion != "" {
		commands = append(commands,
			"curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y",
			"source $HOME/.cargo/env",
		)
		if rustVersion != "latest" {
			commands = append(commands, fmt.Sprintf("rustup install %s", rustVersion))
		}
	}

	if pythonVersion, ok := env["INSTALL_PYTHON"]; ok && pythonVersion != "" {
		if pythonVersion == "latest" || pythonVersion == "3" {
			commands = append(commands, "apt-get update && apt-get install -y python3 python3-pip")
		} else {
			commands = append(commands, fmt.Sprintf("apt-get update && apt-get install -y python%s python%s-pip", pythonVersion, pythonVersion))
		}
	}

	if goVersion, ok := env["INSTALL_GOLANG"]; ok && goVersion != "" {
		if goVersion == "latest" {
			goVersion = "1.21.0"
		}
		commands = append(commands,
			fmt.Sprintf("wget https://go.dev/dl/go%s.linux-amd64.tar.gz", goVersion),
			fmt.Sprintf("tar -C /usr/local -xzf go%s.linux-amd64.tar.gz", goVersion),
			"export PATH=$PATH:/usr/local/go/bin",
		)
	}

	if javaVersion, ok := env["INSTALL_JAVA"]; ok && javaVersion != "" {
		if javaVersion == "latest" || javaVersion == "11" {
			commands = append(commands, "apt-get update && apt-get install -y openjdk-11-jdk")
		} else {
			commands = append(commands, fmt.Sprintf("apt-get update && apt-get install -y openjdk-%s-jdk", javaVersion))
		}
	}

	return commands
}

func convertToQubeYaml(config *DockerfileConfig) QubeConfig {
	qubeConfig := QubeConfig{
		System:   config.From,
		Isolated: true,
	}

	if len(config.Expose) > 0 {
		qubeConfig.Ports = config.Expose
	}

	if len(config.Cmd) > 0 {
		if len(config.InstallCmds) > 0 {
			allCmds := append(config.InstallCmds, config.Cmd...)
			qubeConfig.Cmd = allCmds
		} else if len(config.Cmd) == 1 {
			qubeConfig.Cmd = config.Cmd[0]
		} else {
			qubeConfig.Cmd = config.Cmd
		}
	}

	if len(config.Env) > 0 {
		var envVars []string
		for key, value := range config.Env {
			envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
		}
		if len(envVars) == 1 {
			qubeConfig.Environment = envVars[0]
		} else {
			qubeConfig.Environment = envVars
		}
	}

	return qubeConfig
}

func buildFromQubeYaml(yamlPath, workDir string, config QubeConfig) error {
	containerID, err := BuildContainer("", workDir, config.System)
	if err != nil {
		return fmt.Errorf("failed to build container: %w", err)
	}

	var cmdVec []string
	switch v := config.Cmd.(type) {
	case string:
		cmdVec = []string{v}
	case []interface{}:
		for _, cmd := range v {
			if str, ok := cmd.(string); ok {
				cmdVec = append(cmdVec, str)
			}
		}
	case []string:
		cmdVec = v
	}

	var envVars []string
	switch v := config.Environment.(type) {
	case string:
		envVars = []string{v}
	case []interface{}:
		for _, env := range v {
			if str, ok := env.(string); ok {
				envVars = append(envVars, str)
			}
		}
	case []string:
		envVars = v
	}

	var volumes [][2]string
	for _, vol := range config.Volumes {
		volumes = append(volumes, [2]string{vol.HostPath, vol.ContainerPath})
	}

	portsStr := ""
	if len(config.Ports) > 0 {
		portsStr = strings.Join(config.Ports, ",")
	}

	tracking.TrackContainerNamed(
		containerID,
		-1,
		workDir,
		cmdVec,
		config.System,
		portsStr,
		config.Isolated,
		volumes,
		envVars,
	)

	color.Green("Container %s built from Dockerfile conversion. It will be started by the daemon.", containerID)
	return nil
}
