package main

import (
	"fmt"
	"os"

	"github.com/Voyrox/Qube/src/cli"
	"github.com/Voyrox/Qube/src/core/container"
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

	if err := cli.Execute(); err != nil {
		color.New(color.FgRed).Fprintf(os.Stderr, "✗ %v\n", err)
		os.Exit(1)
	}
}
