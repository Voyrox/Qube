package daemon

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Voyrox/Qube/internal/api"
	"github.com/Voyrox/Qube/internal/config"
	"github.com/Voyrox/Qube/internal/core/cgroup"
	"github.com/Voyrox/Qube/internal/core/container"
	"github.com/Voyrox/Qube/internal/core/tracking"
	"github.com/fatih/color"
)

var running = true
var restartTimestamps = make(map[string]time.Time)
var restartCounts = make(map[string]int)

func StartDaemon(debug bool) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigChan
		running = false
	}()

	cleanupOrphanedContainers()

	if err := cgroup.InitCgroupRoot(); err != nil {
		color.Yellow("Warning: Failed to initialize cgroup root: %v", err)
		color.Yellow("Containers will run without resource limits.")
	} else if debug {
		color.Green("Cgroup root initialized successfully.")
	}

	go api.StartServer()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for running {
		select {
		case <-ticker.C:
			monitorAndRestartContainers(debug)
		case <-sigChan:
			color.Yellow("Received shutdown signal, stopping daemon...")
			running = false
			return
		}
	}
}

func cleanupOrphanedContainers() {
	trackedContainers := tracking.GetAllTrackedEntries()
	trackedNames := make(map[string]bool)

	for _, entry := range trackedContainers {
		trackedNames[entry.Name] = true
	}

	containersPath := config.QubeContainersBase
	if _, err := os.Stat(containersPath); os.IsNotExist(err) {
		return
	}

	entries, err := ioutil.ReadDir(containersPath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		if dirName == "images" {
			continue
		}

		if !trackedNames[dirName] {
			color.Yellow("Found orphaned container directory: %s. Removing...", dirName)

			procPath := filepath.Join(containersPath, dirName, "rootfs", "proc")
			for {
				mounts, err := ioutil.ReadFile("/proc/mounts")
				if err != nil {
					break
				}

				if !strings.Contains(string(mounts), procPath) {
					break
				}

				syscall.Unmount(procPath, syscall.MNT_DETACH)
				time.Sleep(100 * time.Millisecond)
			}

			path := filepath.Join(containersPath, dirName)
			if err := os.RemoveAll(path); err != nil {
				color.Red("Failed to remove orphaned container %s: %v", dirName, err)
			} else {
				color.Green("✓ Removed orphaned container directory: %s", dirName)
			}
		}
	}
}

func monitorAndRestartContainers(debug bool) {
	entries := tracking.GetAllTrackedEntries()

	for _, entry := range entries {
		if entry.PID > 0 {
			procPath := fmt.Sprintf("/proc/%d", entry.PID)
			if _, err := os.Stat(procPath); os.IsNotExist(err) {
				if debug {
					color.Yellow("Container %s (PID: %d) has exited, restarting...", entry.Name, entry.PID)
				}
				tracking.UpdateContainerPID(entry.Name, -1, entry.Dir, entry.Command, entry.Image, entry.Ports, entry.Isolated, entry.Volumes, entry.EnvVars)

				go restartContainer(&entry, debug)
			} else if debug {
				//if stats, err := cgroup.GetMemoryStats(entry.Name); err == nil {
				//	fmt.Printf("[Monitor] %s: Memory=%.1fMB\n", entry.Name, stats.CurrentMB())
				//}
			}
		} else if entry.PID == -1 {
			lastRestart, exists := restartTimestamps[entry.Name]

			if exists && time.Since(lastRestart) < 10*time.Second {
				count := restartCounts[entry.Name]
				if count >= 3 {
					if debug {
						color.Yellow("Container %s is crash-looping, pausing restarts", entry.Name)
					}
					continue
				}
			}

			if exists && time.Since(lastRestart) > 60*time.Second {
				restartCounts[entry.Name] = 0
			}

			if debug {
				color.Cyan("Restarting exited container: %s", entry.Name)
			}

			restartTimestamps[entry.Name] = time.Now()
			restartCounts[entry.Name]++
			go restartContainer(&entry, debug)
		}
	}

	cgroupRoot := config.CgroupRoot
	if _, err := os.Stat(cgroupRoot); err == nil {
		cgroupDirs, err := ioutil.ReadDir(cgroupRoot)
		if err == nil {
			trackedNames := make(map[string]bool)
			for _, entry := range entries {
				trackedNames[entry.Name] = true
			}

			for _, dir := range cgroupDirs {
				if dir.IsDir() && !trackedNames[dir.Name()] {
					if debug {
						color.Yellow("Removing orphaned cgroup: %s", dir.Name())
					}
					cgroup.RemoveCgroup(dir.Name())
				}
			}
		}
	}
}

func restartContainer(entry *tracking.ContainerEntry, debug bool) {
	if debug {
		color.Green("Starting container %s...", entry.Name)
	}

	err := container.RunContainer(
		entry.Name,
		entry.Dir,
		entry.Command,
		debug,
		entry.Image,
		entry.Ports,
		entry.Isolated,
		entry.Volumes,
		entry.EnvVars,
	)

	if err != nil {
		if debug {
			color.Red("Failed to restart container %s: %v", entry.Name, err)
		}
		return
	}

	if debug {
		color.Green("✓ Container %s restarted", entry.Name)
	}
}
