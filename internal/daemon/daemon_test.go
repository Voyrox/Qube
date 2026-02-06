package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Voyrox/Qube/internal/config"
	"github.com/Voyrox/Qube/internal/core/tracking"
)

func TestCleanupOrphanedContainers(t *testing.T) {
	tmp := t.TempDir()
	origBase := config.QubeContainersBase
	config.QubeContainersBase = tmp
	defer func() { config.QubeContainersBase = origBase }()
	origGetAll := getAllTrackedEntries
	origStat := osStat
	origReadDir := readDir
	defer func() {
		getAllTrackedEntries = origGetAll
		osStat = origStat
		readDir = origReadDir
	}()

	getAllTrackedEntries = func() []tracking.ContainerEntry { return nil }
	readDir = func(path string) ([]os.FileInfo, error) { return origReadDir(path) }

	os.MkdirAll(filepath.Join(tmp, "orphan", "rootfs", "proc"), 0755)
	os.WriteFile(filepath.Join(tmp, "orphan", "rootfs", "proc", "dummy"), []byte(""), 0644)

	cleanupOrphanedContainers()

	if _, err := os.Stat(filepath.Join(tmp, "orphan")); err == nil {
		t.Fatalf("expected orphan removed")
	}
}

func TestMonitorAndRestartContainersNoProc(t *testing.T) {
	called := make(chan struct{}, 1)
	origRestart := restartContainerFn
	restartContainerFn = func(entry *tracking.ContainerEntry, debug bool) {
		called <- struct{}{}
	}
	defer func() { restartContainerFn = origRestart }()

	origEntries := getAllTrackedEntries
	getAllTrackedEntries = func() []tracking.ContainerEntry {
		return []tracking.ContainerEntry{{Name: "c1", PID: 99999, Dir: "/d", Command: []string{"echo"}}}
	}
	defer func() { getAllTrackedEntries = origEntries }()

	origStat := osStat
	osStat = func(path string) (os.FileInfo, error) {
		if filepath.Base(path) == "99999" {
			return nil, os.ErrNotExist
		}
		return origStat(path)
	}
	defer func() { osStat = origStat }()

	origReadDir := readDir
	readDir = func(path string) ([]os.FileInfo, error) {
		if path == config.CgroupRoot {
			return []os.FileInfo{}, nil
		}
		return origReadDir(path)
	}
	defer func() { readDir = origReadDir }()

	monitorAndRestartContainers(false)
	select {
	case <-called:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("expected restart invocation")
	}
}
