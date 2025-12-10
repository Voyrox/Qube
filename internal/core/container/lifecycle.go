package container

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Voyrox/Qube/internal/config"
	"github.com/Voyrox/Qube/internal/core/cgroup"
	"github.com/Voyrox/Qube/internal/core/tracking"
	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

func GenerateContainerID() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return fmt.Sprintf("Qube-%s", string(b))
}

func BuildContainer(existingName, workDir, image string) (string, error) {
	containerID := existingName
	if containerID == "" {
		containerID = GenerateContainerID()
	}

	rootfs := GetRootfs(containerID)
	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		bar := progressbar.NewOptions(4,
			progressbar.OptionSetDescription("Building container"),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "∷",
				SaucerPadding: "∷",
				BarStart:      "[",
				BarEnd:        "]",
			}),
		)

		bar.Describe("Preparing container filesystem...")
		if err := PrepareRootfsDir(containerID); err != nil {
			return containerID, err
		}
		bar.Add(1)

		bar.Describe("Extracting container image...")
		if err := ExtractRootfsTar(containerID, image); err != nil {
			bar.Finish()
			color.Red("Error: Invalid image provided ('%s'). Reason: %v", image, err)
			tracking.RemoveContainerFromTrackingByName(containerID)
			return containerID, err
		}
		bar.Add(1)

		bar.Describe("Copying working directory...")
		if err := CopyDirectoryIntoHome(containerID, workDir); err != nil {
			return containerID, err
		}
		bar.Add(1)

		bar.Describe("Container build complete!")
		bar.Add(1)
		bar.Finish()
	} else {
		fmt.Println("Container filesystem already exists. Skipping build.")
	}

	return containerID, nil
}

func ListContainers() {
	entries := tracking.GetAllTrackedEntries()
	if len(entries) == 0 {
		fmt.Println()
		color.New(color.Faint).Println("  No containers running")
		color.Blue("  → Use 'qube run' to start a container")
		fmt.Println()
		return
	}

	color.New(color.Bold, color.FgWhite).Println("\n  CONTAINERS")
	color.New(color.Faint).Println("  ───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────")

	for _, entry := range entries {
		procPath := fmt.Sprintf("/proc/%d", entry.PID)

		var statusIcon string
		var statusText *color.Color

		if entry.PID > 0 {
			if _, err := os.Stat(procPath); err == nil {
				statusIcon = "●"
				statusText = color.New(color.FgGreen)
			} else {
				statusIcon = "▲"
				statusText = color.New(color.FgYellow)
			}
		} else if entry.PID == -2 {
			statusIcon = "■"
			statusText = color.New(color.FgRed)
		} else {
			statusIcon = "▲"
			statusText = color.New(color.FgYellow)
		}

		var status string
		if entry.PID > 0 {
			if _, err := os.Stat(procPath); err == nil {
				status = "running"
			} else {
				status = "exited"
			}
		} else if entry.PID == -2 {
			status = "stopped"
		} else {
			status = "exited"
		}

		var memStr string
		if stats, err := cgroup.GetMemoryStats(entry.Name); err == nil {
			mb := stats.CurrentMB()
			if mb < 100.0 {
				memStr = color.GreenString("%.1fM", mb)
			} else if mb < 1024.0 {
				memStr = color.YellowString("%.0fM", mb)
			} else {
				memStr = color.RedString("%.1fG", mb/1024.0)
			}
		} else {
			if entry.PID > 0 {
				if mem, err := cgroup.GetMemoryFromProc(entry.PID); err == nil && mem > 0 {
					mb := float64(mem) / (1024.0 * 1024.0)
					memStr = fmt.Sprintf("%.1fM", mb)
				} else {
					memStr = "N/A"
				}
			} else {
				memStr = "N/A"
			}
		}

		cpuStr := "N/A"
		if entry.PID > 0 {
			if _, err := os.Stat(procPath); err == nil {
				if cpu, err := cgroup.GetCPUFromProc(entry.PID); err == nil {
					if cpu < 50.0 {
						cpuStr = color.GreenString("%.1f%%", cpu)
					} else if cpu < 80.0 {
						cpuStr = color.YellowString("%.1f%%", cpu)
					} else {
						cpuStr = color.RedString("%.1f%%", cpu)
					}
				}
			}
		}

		uptime := "N/A"
		if entry.PID > 0 {
			if uptimeSecs, err := tracking.GetProcessUptime(entry.PID); err == nil {
				uptime = formatUptime(uptimeSecs)
			}
		}

		cmdStr := strings.Join(entry.Command, " ")
		if len(cmdStr) > 40 {
			cmdStr = cmdStr[:37] + "..."
		}

		fmt.Printf("  %s %s %-15s PID: %-8d Mem: %-10s CPU: %-10s Uptime: %s\n",
			statusIcon,
			statusText.Sprint(status),
			entry.Name,
			entry.PID,
			memStr,
			cpuStr,
			uptime,
		)

		fmt.Printf("    %s %s\n", color.New(color.Faint).Sprint("cmd:"), color.CyanString(cmdStr))

		if entry.Ports != "" && entry.Ports != "none" {
			fmt.Printf("    %s %s\n", color.New(color.Faint).Sprint("ports:"), color.MagentaString(entry.Ports))
		}

		if entry.Isolated {
			fmt.Printf("    %s %s\n", color.New(color.Faint).Sprint("isolation:"), color.YellowString("enabled"))
		}

		fmt.Println()
	}
}

func formatUptime(seconds uint64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	} else if seconds < 3600 {
		return fmt.Sprintf("%dm", seconds/60)
	} else if seconds < 86400 {
		return fmt.Sprintf("%dh %dm", seconds/3600, (seconds%3600)/60)
	} else {
		return fmt.Sprintf("%dd %dh", seconds/86400, (seconds%86400)/3600)
	}
}

func StopContainer(pid int) error {
	entries := tracking.GetAllTrackedEntries()
	for _, entry := range entries {
		if entry.PID == pid {
			if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
				return fmt.Errorf("failed to kill process %d: %w", pid, err)
			}

			tracking.UpdateContainerPID(entry.Name, -2, entry.Dir, entry.Command, entry.Image, entry.Ports, entry.Isolated, entry.Volumes, entry.EnvVars)
			color.Green("✓ Container %s (PID: %d) stopped", entry.Name, pid)
			return nil
		}
	}

	return fmt.Errorf("container with PID %d not found", pid)
}

func DeleteContainer(nameOrPID string) error {
	entries := tracking.GetAllTrackedEntries()

	for _, entry := range entries {
		if entry.Name == nameOrPID || fmt.Sprintf("%d", entry.PID) == nameOrPID {
			if entry.PID > 0 {
				syscall.Kill(entry.PID, syscall.SIGKILL)
				time.Sleep(500 * time.Millisecond)
			}

			tracking.RemoveContainerFromTrackingByName(entry.Name)

			cgroup.RemoveCgroup(entry.Name)

			containerDir := filepath.Join(config.QubeContainersBase, entry.Name)
			rootfsDir := filepath.Join(containerDir, "rootfs")

			for _, mount := range []string{"proc", "sys", "dev/pts", "dev"} {
				mountPath := filepath.Join(rootfsDir, mount)
				for i := 0; i < 5; i++ {
					syscall.Unmount(mountPath, syscall.MNT_DETACH)
					time.Sleep(50 * time.Millisecond)
				}
			}

			if err := os.RemoveAll(containerDir); err != nil {
				return fmt.Errorf("failed to remove container directory: %w", err)
			}

			color.Green("✓ Container %s deleted", entry.Name)
			return nil
		}
	}

	return fmt.Errorf("container %s not found", nameOrPID)
}
