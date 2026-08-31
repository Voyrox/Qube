package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Voyrox/Qube/src/core/cgroup"
	"github.com/Voyrox/Qube/src/core/container"
	"github.com/Voyrox/Qube/src/core/tracking"
	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
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

	noColor bool
)

type QMLConfig struct {
	Container struct {
		System      string                   `yaml:"system"`
		Ports       []string                 `yaml:"ports"`
		Cmd         []string                 `yaml:"cmd"`
		Isolated    bool                     `yaml:"isolated"`
		Environment map[string]string        `yaml:"environment"`
		Volumes     []container.VolumeConfig `yaml:"volumes"`
		Debug       bool                     `yaml:"debug"`
	} `yaml:"container"`
}

type runOptions struct {
	image     string
	ports     string
	isolated  bool
	debug     bool
	volumes   []string
	envVars   []string
	cmd       []string
	cmdString string
}

// Execute runs the Qube command-line interface.
func Execute() error {
	rootCmd := newRootCommand()
	return rootCmd.Execute()
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "qube",
		Short:         "Lightweight Linux container runtime",
		Long:          "Qube is a lightweight Linux container runtime and container manager written in Go.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			color.NoColor = noColor
			return requireSupportedHost()
		},
	}

	cmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	cmd.AddCommand(
		newRunCommand(),
		newListCommand(),
		newInfoCommand(),
		newStopCommand(),
		newStartCommand(),
		newDeleteCommand(),
		newEvalCommand(),
		newSnapshotCommand(),
		newDockerCommand(),
		newPullCommand(),
	)

	cmd.SetHelpCommand(&cobra.Command{Hidden: true})
	cmd.SetUsageTemplate(usageTemplate())
	cmd.SetHelpTemplate(helpTemplate())
	return cmd
}

func requireSupportedHost() error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("native container isolation is not fully supported on Windows")
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("this program must be run as root; try running it with sudo")
	}
	return nil
}

func newRunCommand() *cobra.Command {
	opts := runOptions{}
	cmd := &cobra.Command{
		Use:     "run [flags] --cmd <command>",
		Short:   "Create and run a new container",
		Example: "  qube run --image Voyrox:nodejs:25.2.0 --ports 3000 --cmd \"npm install && npm start\"\n  qube run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.cmdString != "" {
				opts.cmd = []string{opts.cmdString}
			} else {
				opts.cmd = args
			}
			return runCommand(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.image, "image", "i", "", "image in <user>:<image>:<version> format")
	cmd.Flags().StringVarP(&opts.ports, "ports", "p", "", "port mapping or comma-separated port mappings")
	cmd.Flags().StringArrayVarP(&opts.envVars, "env", "e", nil, "environment variable in KEY=VALUE format")
	cmd.Flags().StringArrayVarP(&opts.volumes, "volume", "v", nil, "bind mount in /host/path:/container/path format")
	cmd.Flags().StringVar(&opts.cmdString, "cmd", "", "command to run inside the container")
	cmd.Flags().BoolVar(&opts.isolated, "isolated", false, "run with an isolated network namespace")
	cmd.Flags().BoolVar(&opts.debug, "debug", false, "show runtime debug output")
	return cmd
}

func newListCommand() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "ps"},
		Short:   "List containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listCommand(jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print containers as JSON")
	return cmd
}

func newInfoCommand() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "info <containerName|pid>",
		Short: "Show detailed container information",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return infoCommand(args[0], jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print container information as JSON")
	return cmd
}

func newStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <pid|containerName>",
		Short: "Stop a running container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return stopCommand(args[0])
		},
	}
}

func newStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "start <pid|containerName>",
		Short: "Start a stopped container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := container.StartContainer(args[0]); err != nil {
				return fmt.Errorf("failed to start container: %w", err)
			}
			return nil
		},
	}
}

func newDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <pid|containerName>",
		Aliases: []string{"rm"},
		Short:   "Delete a container",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := deleteContainerFn(args[0]); err != nil {
				return fmt.Errorf("failed to delete container: %w", err)
			}
			return nil
		},
	}
}

func newEvalCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "eval <containerName|pid> [command]",
		Aliases: []string{"exec"},
		Short:   "Execute a command in a container",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return evalCommand(args[0], args[1:])
		},
	}
}

func newSnapshotCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "snapshot <containerName|pid>",
		Short: "Create a snapshot of a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return snapshotCommand(args[0])
		},
	}
}

func newDockerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "docker <dockerfilePath>",
		Short: "Convert and run a Dockerfile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat(args[0]); os.IsNotExist(err) {
				return fmt.Errorf("dockerfile not found: %s", args[0])
			}
			if err := convertAndRunFn(args[0]); err != nil {
				return fmt.Errorf("failed to convert Dockerfile: %w", err)
			}
			return nil
		},
	}
}

func newPullCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "pull <user>:<image>:<version>",
		Short:   "Download an image from Qube Hub",
		Example: "  qube pull Voyrox:nodejs:1.1.0",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return pullCommand(args[0])
		},
	}
}

func runCommand(opts runOptions) error {
	if _, err := os.Stat("qube.yml"); err == nil && opts.image == "" && len(opts.cmd) == 0 {
		return runFromQML()
	}

	if opts.image == "" {
		return fmt.Errorf("--image is required unless running from qube.yml")
	}
	if len(opts.cmd) == 0 {
		return fmt.Errorf("--cmd is required unless running from qube.yml")
	}

	volumes, err := parseVolumes(opts.volumes)
	if err != nil {
		return err
	}
	if err := validateEnvVars(opts.envVars); err != nil {
		return err
	}
	if err := validateImageFn(opts.image); err != nil {
		return fmt.Errorf("invalid image %q: %w", opts.image, err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	return runContainerFn("", cwd, opts.cmd, opts.debug, opts.image, opts.ports, opts.isolated, volumes, opts.envVars)
}

func runFromQML() error {
	data, err := os.ReadFile("qube.yml")
	if err != nil {
		return fmt.Errorf("failed to read qube.yml: %w", err)
	}

	var config QMLConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse qube.yml: %w", err)
	}

	var volumes [][2]string
	for _, vol := range config.Container.Volumes {
		if vol.HostPath != "" && vol.ContainerPath != "" {
			volumes = append(volumes, [2]string{vol.HostPath, vol.ContainerPath})
		}
	}

	var envVars []string
	for k, v := range config.Container.Environment {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	ports := strings.Join(config.Container.Ports, ",")
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	cmdStr := strings.Join(config.Container.Cmd, " && ")
	if cmdStr == "" {
		return fmt.Errorf("qube.yml container.cmd cannot be empty")
	}

	return runContainerFn("", cwd, []string{cmdStr}, config.Container.Debug, config.Container.System, ports, config.Container.Isolated, volumes, envVars)
}

func listCommand(jsonOutput bool) error {
	entries := getAllTrackedEntries()
	rows := make([]containerRow, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, buildContainerRow(entry, false))
	}

	if jsonOutput {
		return writeJSON(rows)
	}

	if len(rows) == 0 {
		fmt.Println()
		muted.Println("  No containers found")
		info.Println("  → Use 'qube run' to start a container")
		fmt.Println()
		return nil
	}

	fmt.Println()
	title.Println("  Qube containers")
	fmt.Println()

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleRounded)
	t.Style().Options.SeparateRows = false
	t.AppendHeader(table.Row{"Status", "Name", "Image", "PID", "Ports", "Mem", "CPU", "Uptime"})
	for _, entry := range entries {
		row := buildContainerRow(entry, true)
		t.AppendRow(table.Row{row.Status, row.Name, row.Image, row.PID, row.Ports, row.Memory, row.CPU, row.Uptime})
	}
	t.Render()
	fmt.Println()
	muted.Println("  Run 'qube info <name>' for details.")
	fmt.Println()
	return nil
}

func infoCommand(nameOrPID string, jsonOutput bool) error {
	entry, ok := findContainerEntry(nameOrPID)
	if !ok {
		return fmt.Errorf("container %s not found", nameOrPID)
	}

	row := buildContainerRow(entry, jsonOutput == false)
	if jsonOutput {
		return writeJSON(containerDetails{
			Status:           containerStatus(entry),
			Name:             entry.Name,
			PID:              row.PID,
			Image:            entry.Image,
			Created:          formatTimestamp(entry.Timestamp),
			Uptime:           row.Uptime,
			Network:          formatNetwork(entry.Isolated),
			Ports:            emptyDash(entry.Ports),
			Command:          entry.Command,
			WorkingDirectory: entry.Dir,
			Volumes:          entry.Volumes,
			Environment:      entry.EnvVars,
		})
	}

	fmt.Println()
	title.Printf("  Container %s\n", entry.Name)
	fmt.Println()
	printField("Status", row.Status)
	printField("PID", row.PID)
	printField("Image", entry.Image)
	printField("Created", formatTimestamp(entry.Timestamp))
	printField("Uptime", row.Uptime)
	printField("Network", formatNetwork(entry.Isolated))
	printField("Ports", emptyDash(entry.Ports))
	fmt.Println()
	printSection("Command", strings.Join(entry.Command, " "))
	printSection("Working directory", entry.Dir)
	if len(entry.Volumes) > 0 {
		printSection("Volumes", formatVolumes(entry.Volumes))
	}
	if len(entry.EnvVars) > 0 {
		printSection("Environment", strings.Join(entry.EnvVars, "\n"))
	}
	fmt.Println()
	return nil
}

func stopCommand(nameOrPID string) error {
	pid, err := strconv.Atoi(nameOrPID)
	if err == nil {
		if err := stopContainerFn(pid); err != nil {
			return fmt.Errorf("failed to stop container: %w", err)
		}
		return nil
	}

	entry, ok := findContainerEntry(nameOrPID)
	if !ok {
		return fmt.Errorf("container %s not found", nameOrPID)
	}
	if err := stopContainerFn(entry.PID); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	return nil
}

func evalCommand(nameOrPID string, commandArgs []string) error {
	entry, ok := findContainerEntry(nameOrPID)
	if !ok {
		return fmt.Errorf("container %s not found", nameOrPID)
	}

	rootfs := container.GetRootfs(entry.Name)
	cmd := "sh"
	if len(commandArgs) > 0 {
		cmd = strings.Join(commandArgs, " ")
	}

	execCmd := exec.Command("nsenter", "-t", fmt.Sprintf("%d", entry.PID), "-m", "-u", "-i", "-p", "chroot", rootfs, "/bin/sh", "-c", cmd)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	return execCmd.Run()
}

func snapshotCommand(nameOrPID string) error {
	entry, ok := findContainerEntry(nameOrPID)
	if !ok {
		return fmt.Errorf("container %s not found", nameOrPID)
	}

	snapshotPath := filepath.Join(entry.Dir, fmt.Sprintf("snapshot_%d.tar.gz", time.Now().Unix()))
	info.Printf("Creating snapshot of %s...\n", entry.Name)

	rootfs := container.GetRootfs(entry.Name)
	cmd := exec.Command("tar", "-czf", snapshotPath, "-C", filepath.Dir(rootfs), "rootfs")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	success.Printf("✓ Snapshot created: %s\n", snapshotPath)
	return nil
}

func pullCommand(imageSpec string) error {
	parts := strings.Split(imageSpec, ":")
	if len(parts) != 3 {
		return fmt.Errorf("image must be in format <user>:<image>:<version>; example: Voyrox:nodejs:1.1.0")
	}

	user, image, version := parts[0], parts[1], parts[2]
	info.Printf("Pulling image %s/%s version %s from Qube Hub...\n", user, image, version)
	if err := pullImageFromHubFn(user, image, version); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	success.Printf("✓ Successfully pulled %s:%s:%s\n", user, image, version)
	return nil
}

func parseVolumes(raw []string) ([][2]string, error) {
	volumes := make([][2]string, 0, len(raw))
	for _, value := range raw {
		parts := strings.SplitN(value, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("--volume must be in /host/path:/container/path format")
		}
		volumes = append(volumes, [2]string{parts[0], parts[1]})
	}
	return volumes, nil
}

func validateEnvVars(envVars []string) error {
	for _, env := range envVars {
		if !strings.Contains(env, "=") {
			return fmt.Errorf("--env must be in KEY=VALUE format")
		}
	}
	return nil
}

func findContainerEntry(nameOrPID string) (tracking.ContainerEntry, bool) {
	for _, entry := range getAllTrackedEntries() {
		if entry.Name == nameOrPID || fmt.Sprintf("%d", entry.PID) == nameOrPID {
			return entry, true
		}
	}
	return tracking.ContainerEntry{}, false
}

type containerRow struct {
	Status string `json:"status"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	PID    string `json:"pid"`
	Ports  string `json:"ports"`
	Memory string `json:"memory"`
	CPU    string `json:"cpu"`
	Uptime string `json:"uptime"`
}

type containerDetails struct {
	Status           string      `json:"status"`
	Name             string      `json:"name"`
	PID              string      `json:"pid"`
	Image            string      `json:"image"`
	Created          string      `json:"created"`
	Uptime           string      `json:"uptime"`
	Network          string      `json:"network"`
	Ports            string      `json:"ports"`
	Command          []string    `json:"command"`
	WorkingDirectory string      `json:"working_directory"`
	Volumes          [][2]string `json:"volumes"`
	Environment      []string    `json:"environment"`
}

func buildContainerRow(entry tracking.ContainerEntry, decorated bool) containerRow {
	status := containerStatus(entry)
	return containerRow{
		Status: formatStatus(status, decorated),
		Name:   entry.Name,
		Image:  shortenImage(entry.Image),
		PID:    formatPID(entry.PID, status),
		Ports:  emptyDash(entry.Ports),
		Memory: memoryUsage(entry),
		CPU:    cpuUsage(entry, status),
		Uptime: uptime(entry, status),
	}
}

func containerStatus(entry tracking.ContainerEntry) string {
	if entry.PID == -2 {
		return "stopped"
	}
	if entry.PID <= 0 {
		return "exited"
	}
	if _, err := os.Stat(fmt.Sprintf("/proc/%d", entry.PID)); err == nil {
		return "running"
	}
	return "exited"
}

func formatStatus(status string, decorated bool) string {
	if !decorated {
		return status
	}
	switch status {
	case "running":
		return color.GreenString("● running")
	case "stopped":
		return color.RedString("■ stopped")
	default:
		return color.YellowString("▲ exited")
	}
}

func formatPID(pid int, status string) string {
	if status != "running" || pid <= 0 {
		return "-"
	}
	return strconv.Itoa(pid)
}

func memoryUsage(entry tracking.ContainerEntry) string {
	if stats, err := cgroup.GetMemoryStats(entry.Name); err == nil {
		mb := stats.CurrentMB()
		if mb < 100.0 {
			return color.GreenString("%.1fM", mb)
		}
		if mb < 1024.0 {
			return color.YellowString("%.0fM", mb)
		}
		return color.RedString("%.1fG", mb/1024.0)
	}

	if entry.PID > 0 {
		if mem, err := cgroup.GetMemoryFromProc(entry.PID); err == nil && mem > 0 {
			return fmt.Sprintf("%.1fM", float64(mem)/(1024.0*1024.0))
		}
	}
	return "-"
}

func cpuUsage(entry tracking.ContainerEntry, status string) string {
	if status != "running" || entry.PID <= 0 {
		return "-"
	}
	cpu, err := cgroup.GetCPUFromProc(entry.PID)
	if err != nil {
		return "-"
	}
	if cpu < 50.0 {
		return color.GreenString("%.1f%%", cpu)
	}
	if cpu < 80.0 {
		return color.YellowString("%.1f%%", cpu)
	}
	return color.RedString("%.1f%%", cpu)
}

func uptime(entry tracking.ContainerEntry, status string) string {
	if status != "running" || entry.PID <= 0 {
		return "-"
	}
	seconds, err := getProcessUptimeFn(entry.PID)
	if err != nil {
		return "-"
	}
	return formatUptime(seconds)
}

func formatUptime(seconds uint64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	if seconds < 86400 {
		return fmt.Sprintf("%dh %dm", seconds/3600, (seconds%3600)/60)
	}
	return fmt.Sprintf("%dd %dh", seconds/86400, (seconds%86400)/3600)
}

func formatTimestamp(timestamp uint64) string {
	if timestamp == 0 {
		return "-"
	}
	return time.Unix(int64(timestamp), 0).Format("2006-01-02 15:04:05")
}

func formatNetwork(isolated bool) string {
	if isolated {
		return "isolated"
	}
	return "host"
}

func formatVolumes(volumes [][2]string) string {
	lines := make([]string, 0, len(volumes))
	for _, volume := range volumes {
		lines = append(lines, fmt.Sprintf("%s → %s", volume[0], volume[1]))
	}
	return strings.Join(lines, "\n")
}

func shortenImage(image string) string {
	if len(image) <= 28 {
		return emptyDash(image)
	}
	return image[:25] + "..."
}

func emptyDash(value string) string {
	if value == "" || value == "none" {
		return "-"
	}
	return value
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

var (
	title   = color.New(color.FgCyan, color.Bold)
	success = color.New(color.FgGreen)
	info    = color.New(color.FgBlue)
	muted   = color.New(color.Faint)
	label   = color.New(color.Faint)
)

func printField(name, value string) {
	fmt.Printf("  %-18s %s\n", label.Sprint(name), value)
}

func printSection(name, value string) {
	label.Printf("  %s\n", name)
	for _, line := range strings.Split(value, "\n") {
		fmt.Printf("    %s\n", line)
	}
	fmt.Println()
}

func usageTemplate() string {
	return `Usage:
  {{.UseLine}}

{{- if .HasAvailableAliases}}
Aliases:
  {{.NameAndAliases}}
{{- end}}

{{- if .HasAvailableSubCommands}}
Commands:
{{- range .Commands}}
{{- if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}
{{- end}}
{{- end}}
{{- end}}

{{- if .HasAvailableLocalFlags}}
Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
{{- end}}

{{- if .HasAvailableInheritedFlags}}
Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}
{{- end}}
`
}

func helpTemplate() string {
	return `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if .HasExample}}Examples:
{{.Example}}

{{end}}{{.UsageString}}`
}
