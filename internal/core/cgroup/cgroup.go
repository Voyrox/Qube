package cgroup

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Voyrox/Qube/internal/config"
	"github.com/fatih/color"
)

type cpuCache struct {
	totalTime uint64
	timestamp time.Time
}

var cpuCacheMap = make(map[int]*cpuCache)

const (
	MemoryMax     = config.MemoryMaxMB * 1024 * 1024
	MemorySwapMax = config.MemorySwapMaxMB * 1024 * 1024
)

type MemoryStats struct {
	CurrentBytes uint64
}

func (m *MemoryStats) CurrentMB() float64 {
	return float64(m.CurrentBytes) / (1024.0 * 1024.0)
}

func InitCgroupRoot() error {
	if _, err := os.Stat(config.CgroupRoot); os.IsNotExist(err) {
		if err := os.MkdirAll(config.CgroupRoot, 0755); err != nil {
			return err
		}
	}

	subtreeControl := filepath.Join(config.CgroupRoot, "cgroup.subtree_control")
	if err := ioutil.WriteFile(subtreeControl, []byte("+memory +cpu"), 0644); err != nil {
		fmt.Printf("Warning: Failed to enable cgroup controllers: %v\n", err)
	} else {
		color.Green("✓ Cgroup controllers enabled: memory, cpu")
	}

	return nil
}

func SetupCgroupForContainer(containerName string) (string, error) {
	if err := InitCgroupRoot(); err != nil {
		return "", err
	}

	cgroupPath := filepath.Join(config.CgroupRoot, containerName)

	if _, err := os.Stat(cgroupPath); os.IsNotExist(err) {
		if err := os.MkdirAll(cgroupPath, 0755); err != nil {
			return "", err
		}
	}

	memMaxPath := filepath.Join(cgroupPath, "memory.max")
	if err := ioutil.WriteFile(memMaxPath, []byte(fmt.Sprintf("%d", MemoryMax)), 0644); err != nil {
		fmt.Printf("Warning: Failed to set memory.max limit: %v\n", err)
	} else {
		color.Green("✓ Memory limit set: %d bytes (%d MB)", MemoryMax, config.MemoryMaxMB)
	}

	memSwapPath := filepath.Join(cgroupPath, "memory.swap.max")
	if err := ioutil.WriteFile(memSwapPath, []byte(fmt.Sprintf("%d", MemorySwapMax)), 0644); err != nil {
		fmt.Printf("Warning: Failed to set memory.swap.max limit: %v\n", err)
	} else {
		color.Green("✓ Swap limit set: %d bytes (%d MB)", MemorySwapMax, config.MemorySwapMaxMB)
	}

	cpuMaxPath := filepath.Join(cgroupPath, "cpu.max")
	cpuLimit := fmt.Sprintf("%d %d", config.CPUQuotaUS, config.CPUPeriodUS)
	if err := ioutil.WriteFile(cpuMaxPath, []byte(cpuLimit), 0644); err != nil {
		fmt.Printf("Warning: Failed to set cpu.max limit: %v\n", err)
	} else {
		color.Green("✓ CPU limit set: %d/%d", config.CPUQuotaUS, config.CPUPeriodUS)
	}

	return cgroupPath, nil
}

func AddProcessToCgroup(cgroupPath string, pid int) error {
	procsPath := filepath.Join(cgroupPath, "cgroup.procs")
	return ioutil.WriteFile(procsPath, []byte(fmt.Sprintf("%d", pid)), 0644)
}

func GetMemoryStats(containerName string) (*MemoryStats, error) {
	cgroupPath := filepath.Join(config.CgroupRoot, containerName)
	memCurrentPath := filepath.Join(cgroupPath, "memory.current")

	content, err := ioutil.ReadFile(memCurrentPath)
	if err != nil {
		return nil, err
	}

	currentBytes, err := strconv.ParseUint(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		return nil, err
	}

	return &MemoryStats{CurrentBytes: currentBytes}, nil
}

func GetMemoryFromProc(pid int) (uint64, error) {
	statusPath := fmt.Sprintf("/proc/%d/status", pid)
	content, err := ioutil.ReadFile(statusPath)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				rss, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					return rss * 1024, nil
				}
			}
		}
	}

	return 0, fmt.Errorf("VmRSS not found")
}

func GetCPUFromProc(pid int) (float64, error) {
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	content, err := ioutil.ReadFile(statPath)
	if err != nil {
		delete(cpuCacheMap, pid)
		return 0, err
	}

	fields := strings.Fields(string(content))
	if len(fields) < 15 {
		return 0, fmt.Errorf("invalid stat format")
	}

	utime, _ := strconv.ParseUint(fields[13], 10, 64)
	stime, _ := strconv.ParseUint(fields[14], 10, 64)
	totalTime := utime + stime

	now := time.Now()

	cached, exists := cpuCacheMap[pid]
	if !exists {
		cpuCacheMap[pid] = &cpuCache{
			totalTime: totalTime,
			timestamp: now,
		}
		return 0.0, nil
	}

	timeDelta := now.Sub(cached.timestamp).Seconds()
	if timeDelta < 0.1 {
		return 0.0, nil
	}

	cpuDelta := totalTime - cached.totalTime
	cpuPercent := (float64(cpuDelta) / 100.0 / timeDelta) * 100.0

	cpuCacheMap[pid] = &cpuCache{
		totalTime: totalTime,
		timestamp: now,
	}

	return cpuPercent, nil
}

func RemoveCgroup(containerName string) error {
	cgroupPath := filepath.Join(config.CgroupRoot, containerName)
	return os.RemoveAll(cgroupPath)
}
