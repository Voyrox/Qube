package container

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Voyrox/Qube/internal/core/tracking"
	"gopkg.in/yaml.v3"
)

type CustomConfig struct {
	System      string         `yaml:"system"`
	Cmd         CmdValue       `yaml:"cmd"`
	Ports       []string       `yaml:"ports,omitempty"`
	Environment EnvValue       `yaml:"enviroment,omitempty"`
	Isolated    *bool          `yaml:"isolated,omitempty"`
	Volumes     []VolumeConfig `yaml:"volumes,omitempty"`
}

type CmdValue struct {
	Single string
	List   []string
}

func (c *CmdValue) UnmarshalYAML(value *yaml.Node) error {
	var single string
	if err := value.Decode(&single); err == nil {
		c.Single = single
		c.List = nil
		return nil
	}

	var list []string
	if err := value.Decode(&list); err == nil {
		c.List = list
		c.Single = ""
		return nil
	}

	return fmt.Errorf("cmd must be a string or array of strings")
}

func (c CmdValue) MarshalYAML() (interface{}, error) {
	if len(c.List) > 0 {
		return c.List, nil
	}
	return c.Single, nil
}

func (c CmdValue) AsSlice() []string {
	if len(c.List) > 0 {
		return c.List
	}
	if c.Single != "" {
		return []string{c.Single}
	}
	return []string{}
}

func (c CmdValue) AsString() string {
	if len(c.List) > 0 {
		return strings.Join(c.List, " && ")
	}
	return c.Single
}

type EnvValue struct {
	Single string
	List   []string
}

func (e *EnvValue) UnmarshalYAML(value *yaml.Node) error {
	var single string
	if err := value.Decode(&single); err == nil {
		e.Single = single
		e.List = nil
		return nil
	}

	var list []string
	if err := value.Decode(&list); err == nil {
		e.List = list
		e.Single = ""
		return nil
	}

	return fmt.Errorf("environment must be a string or array of strings")
}

func (e EnvValue) MarshalYAML() (interface{}, error) {
	if len(e.List) > 0 {
		return e.List, nil
	}
	return e.Single, nil
}

func (e EnvValue) AsSlice() []string {
	if len(e.List) > 0 {
		return e.List
	}
	if e.Single != "" {
		return []string{e.Single}
	}
	return []string{}
}

func ReadQubeYaml(path string) (*CustomConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read qube.yml: %w", err)
	}

	var config CustomConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse qube.yml: %w", err)
	}

	return &config, nil
}

func BuildFromYaml(yamlPath string) error {
	config, err := ReadQubeYaml(yamlPath)
	if err != nil {
		return err
	}

	workDir := filepath.Dir(yamlPath)

	if err := ValidateImage(config.System); err != nil {
		return fmt.Errorf("invalid image: %w", err)
	}

	containerID, err := BuildContainer("", workDir, config.System)
	if err != nil {
		return fmt.Errorf("failed to build container: %w", err)
	}

	cmdVec := config.Cmd.AsSlice()

	envVars := config.Environment.AsSlice()

	var volumes [][2]string
	for _, vol := range config.Volumes {
		volumes = append(volumes, [2]string{vol.HostPath, vol.ContainerPath})
	}

	portsStr := ""
	if len(config.Ports) > 0 {
		portsStr = strings.Join(config.Ports, ",")
	}

	isolated := false
	if config.Isolated != nil {
		isolated = *config.Isolated
	}

	tracking.TrackContainerNamed(
		containerID,
		-1,
		workDir,
		cmdVec,
		config.System,
		portsStr,
		isolated,
		volumes,
		envVars,
	)

	return nil
}
