package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/Voyrox/Qube/internal/cli"
	"github.com/Voyrox/Qube/internal/core/container"
	"github.com/Voyrox/Qube/internal/daemon"
	"github.com/fatih/color"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "__container_init__" {
		if err := container.ContainerInit(); err != nil {
			fmt.Fprintf(os.Stderr, "Container init failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if runtime.GOOS == "windows" {
		color.Red("Warning: Native container isolation is not fully supported on Windows.")
		os.Exit(1)
	}

	if os.Geteuid() != 0 {
		color.New(color.FgRed, color.Bold).Println("Error: This program must be run as root (use sudo).")
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "daemon":
		debug := false
		for _, arg := range os.Args {
			if arg == "--debug" {
				debug = true
				break
			}
		}
		color.New(color.FgGreen, color.Bold).Println("Starting Qubed Daemon...")
		daemon.StartDaemon(debug)

	case "run":
		cli.RunCommand(os.Args)

	case "list":
		cli.ListCommand()

	case "stop":
		cli.StopCommand(os.Args)

	case "start":
		cli.StartCommand(os.Args)

	case "delete":
		cli.DeleteCommand(os.Args)

	case "eval":
		cli.EvalCommand(os.Args)

	case "info":
		cli.InfoCommand(os.Args)

	case "snapshot":
		cli.SnapshotCommand(os.Args)

	case "docker":
		cli.DockerCommand(os.Args)

	case "pull":
		cli.PullCommand(os.Args)

	default:
		color.Red("✗ Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println()
	color.New(color.FgCyan, color.Bold).Println("Qube - Lightweight Container Runtime")
	fmt.Println()
	color.New(color.FgYellow, color.Bold).Println("USAGE:")
	fmt.Println("  qube <command> [options]")
	fmt.Println()
	color.New(color.FgYellow, color.Bold).Println("COMMANDS:")

	commands := []struct {
		name string
		desc string
	}{
		{"daemon", "Start the Qube daemon"},
		{"run", "Create and run a new container"},
		{"list", "List all containers"},
		{"info", "Show detailed container information"},
		{"stop", "Stop a running container"},
		{"start", "Start a stopped container"},
		{"delete", "Delete a container"},
		{"eval", "Execute command in a container"},
		{"snapshot", "Create a snapshot of a container"},
		{"docker", "Convert and run a Dockerfile"},
		{"pull", "Download an image from Qube Hub"},
	}

	for _, cmd := range commands {
		color.New(color.FgGreen).Printf("  %-12s", cmd.name)
		fmt.Printf("%s\n", cmd.desc)
	}

	fmt.Println()
	color.New(color.FgYellow, color.Bold).Println("EXAMPLES:")
	fmt.Println("  qube run --image Ubuntu24_NODE --ports 3000 --cmd \"npm install && npm start\"")
	fmt.Println("  qube list")
	fmt.Println("  qube info myapp")
	fmt.Println("  qube stop myapp")
	fmt.Println()
	color.New(color.FgCyan).Println("Run 'qube <command> --help' for more information on a command.")
	fmt.Println()
}
