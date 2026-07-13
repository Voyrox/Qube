package tracking

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Voyrox/Qube/src/config"
)

type ContainerEntry struct {
	Name      string
	PID       int
	Dir       string
	Command   []string
	Timestamp uint64
	Image     string
	Ports     string
	Isolated  bool
	Volumes   [][2]string
	EnvVars   []string
}

func currentTimestamp() uint64 {
	return uint64(time.Now().Unix())
}

func encodeTrackingField(v any) (string, error) {
	if v == nil {
		return "", nil
	}

	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(raw), nil
}

func decodeVolumesField(raw string) [][2]string {
	if raw == "" {
		return [][2]string{}
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return [][2]string{}
	}

	var volumes [][2]string
	if err := json.Unmarshal(decoded, &volumes); err != nil {
		return [][2]string{}
	}

	return volumes
}

func decodeEnvVarsField(raw string) []string {
	if raw == "" {
		return []string{}
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return []string{}
	}

	var envVars []string
	if err := json.Unmarshal(decoded, &envVars); err != nil {
		return []string{}
	}

	return envVars
}

func TrackContainerNamed(name string, pid int, dir string, cmd []string, image string, ports string, isolated bool, volumes [][2]string, envVars []string) error {
	if err := os.MkdirAll(config.TrackingDir, 0755); err != nil {
		return err
	}

	timestamp := currentTimestamp()
	cmdStr := strings.Join(cmd, "\t")
	encodedVolumes, err := encodeTrackingField(volumes)
	if err != nil {
		return err
	}
	encodedEnvVars, err := encodeTrackingField(envVars)
	if err != nil {
		return err
	}
	line := fmt.Sprintf("%s|%d|%s|%s|%d|%s|%s|%t|%s|%s\n", name, pid, dir, cmdStr, timestamp, image, ports, isolated, encodedVolumes, encodedEnvVars)

	f, err := os.OpenFile(config.ContainerListFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := os.Stat(config.ContainerListFile)
	if err == nil && info.Size() > 0 {
		content, _ := os.ReadFile(config.ContainerListFile)
		if len(content) > 0 && content[len(content)-1] != '\n' {
			f.WriteString("\n")
		}
	}

	_, err = f.WriteString(line)
	return err
}

func UpdateContainerPID(name string, newPID int, newDir string, newCmd []string, image string, ports string, isolated bool, volumes [][2]string, envVars []string) error {
	found := false
	encodedVolumes, err := encodeTrackingField(volumes)
	if err != nil {
		return err
	}
	encodedEnvVars, err := encodeTrackingField(envVars)
	if err != nil {
		return err
	}
	newLine := fmt.Sprintf("%s|%d|%s|%s|%d|%s|%s|%t|%s|%s", name, newPID, newDir, strings.Join(newCmd, "\t"), currentTimestamp(), image, ports, isolated, encodedVolumes, encodedEnvVars)

	content, err := os.ReadFile(config.ContainerListFile)
	if err != nil {
		if os.IsNotExist(err) {
			return TrackContainerNamed(name, newPID, newDir, newCmd, image, ports, isolated, volumes, envVars)
		}
		return err
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			newLines = append(newLines, line)
			continue
		}
		if parts[0] == name {
			newLines = append(newLines, newLine)
			found = true
		} else {
			newLines = append(newLines, line)
		}
	}

	if !found {
		return TrackContainerNamed(name, newPID, newDir, newCmd, image, ports, isolated, volumes, envVars)
	}

	return os.WriteFile(config.ContainerListFile, []byte(strings.Join(newLines, "\n")+"\n"), 0644)
}

func RemoveContainerFromTracking(pid int) error {
	content, err := os.ReadFile(config.ContainerListFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}
		linePID, err := strconv.Atoi(parts[1])
		if err == nil && linePID == pid {
			continue
		}
		newLines = append(newLines, line)
	}

	return os.WriteFile(config.ContainerListFile, []byte(strings.Join(newLines, "\n")+"\n"), 0644)
}

func RemoveContainerFromTrackingByName(name string) error {
	content, err := os.ReadFile(config.ContainerListFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			newLines = append(newLines, line)
			continue
		}
		if parts[0] != name {
			newLines = append(newLines, line)
		}
	}

	return os.WriteFile(config.ContainerListFile, []byte(strings.Join(newLines, "\n")+"\n"), 0644)
}

func GetAllTrackedEntries() []ContainerEntry {
	var entries []ContainerEntry

	file, err := os.Open(config.ContainerListFile)
	if err != nil {
		return entries
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 8 {
			continue
		}

		pid, _ := strconv.Atoi(parts[1])
		timestamp, _ := strconv.ParseUint(parts[4], 10, 64)
		cmdParts := strings.Split(parts[3], "\t")
		isolated := strings.TrimSpace(parts[7]) == "true"

		entry := ContainerEntry{
			Name:      parts[0],
			PID:       pid,
			Dir:       parts[2],
			Command:   cmdParts,
			Timestamp: timestamp,
			Image:     parts[5],
			Ports:     parts[6],
			Isolated:  isolated,
			Volumes: decodeVolumesField(func() string {
				if len(parts) > 8 {
					return parts[8]
				}
				return ""
			}()),
			EnvVars: decodeEnvVarsField(func() string {
				if len(parts) > 9 {
					return parts[9]
				}
				return ""
			}()),
		}
		entries = append(entries, entry)
	}

	return entries
}

func GetProcessUptime(pid int) (uint64, error) {
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	content, err := os.ReadFile(statPath)
	if err != nil {
		return 0, err
	}

	fields := strings.Fields(string(content))
	if len(fields) <= 21 {
		return 0, fmt.Errorf("invalid stat file format")
	}

	startTime, err := strconv.ParseUint(fields[21], 10, 64)
	if err != nil {
		return 0, err
	}

	uptimeContent, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}

	uptimeFields := strings.Fields(string(uptimeContent))
	if len(uptimeFields) < 1 {
		return 0, fmt.Errorf("invalid uptime file")
	}

	systemUptime, err := strconv.ParseFloat(uptimeFields[0], 64)
	if err != nil {
		return 0, err
	}

	clkTck := uint64(100)
	processStartSecs := startTime / clkTck
	uptime := uint64(systemUptime) - processStartSecs

	return uptime, nil
}
