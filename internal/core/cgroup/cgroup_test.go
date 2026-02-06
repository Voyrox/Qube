package cgroup

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Voyrox/Qube/internal/config"
)

func TestInitCgroupRootCreatesDirs(t *testing.T) {
	tmp := t.TempDir()
	origRoot := config.CgroupRoot
	config.CgroupRoot = filepath.Join(tmp, "cg")
	defer func() { config.CgroupRoot = origRoot }()

	if err := InitCgroupRoot(); err != nil {
		t.Fatalf("InitCgroupRoot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(config.CgroupRoot, "cgroup.subtree_control")); err != nil {
		if _, err2 := os.Stat(config.CgroupRoot); err2 != nil {
			t.Fatalf("cgroup root missing: %v", err2)
		}
	}
}

func TestSetupCgroupForContainerWritesLimits(t *testing.T) {
	tmp := t.TempDir()
	origRoot := config.CgroupRoot
	config.CgroupRoot = filepath.Join(tmp, "cg")
	defer func() { config.CgroupRoot = origRoot }()

	path, err := SetupCgroupForContainer("c1")
	if err != nil {
		t.Fatalf("SetupCgroupForContainer: %v", err)
	}
	if !strings.Contains(path, "c1") {
		t.Fatalf("unexpected path: %s", path)
	}
	data, err := os.ReadFile(filepath.Join(path, "memory.max"))
	if err == nil {
		if _, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err != nil {
			t.Fatalf("memory.max parse: %v", err)
		}
	}
}

func TestAddProcessToCgroupWritesPID(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "cgroup.procs")
	if err := AddProcessToCgroup(dir, 1234); err != nil {
		t.Fatalf("AddProcessToCgroup: %v", err)
	}
	b, err := os.ReadFile(pidFile)
	if err != nil || strings.TrimSpace(string(b)) != "1234" {
		t.Fatalf("pid not written: %v %s", err, string(b))
	}
}

func TestGetMemoryStats(t *testing.T) {
	dir := t.TempDir()
	config.CgroupRoot = dir
	defer func() { config.CgroupRoot = "/sys/fs/cgroup/QubeContainers" }()

	os.MkdirAll(filepath.Join(dir, "c1"), 0755)
	os.WriteFile(filepath.Join(dir, "c1", "memory.current"), []byte("2048"), 0644)
	stats, err := GetMemoryStats("c1")
	if err != nil {
		t.Fatalf("GetMemoryStats: %v", err)
	}
	if stats.CurrentBytes != 2048 {
		t.Fatalf("unexpected bytes: %d", stats.CurrentBytes)
	}
}

func TestGetMemoryFromProc(t *testing.T) {
	tmp := t.TempDir()
	status := filepath.Join(tmp, "status")
	os.WriteFile(status, []byte("VmRSS:\t1024 kB\n"), 0644)

	oldReadFile := readFile
	readFile = func(p string) ([]byte, error) {
		if strings.Contains(p, "status") {
			return os.ReadFile(status)
		}
		return nil, errors.New("unexpected")
	}
	defer func() { readFile = oldReadFile }()

	val, err := GetMemoryFromProc(1)
	if err != nil || val != 1024*1024 {
		t.Fatalf("GetMemoryFromProc: %v val=%d", err, val)
	}
}

func TestGetCPUFromProc(t *testing.T) {
	tmp := t.TempDir()
	statPath := filepath.Join(tmp, "stat")
	// utime=100 stime=50
	os.WriteFile(statPath, []byte("1 (cmd) S 0 0 0 0 0 0 0 0 0 100 50 0 0 0 0 0 0 0 0 0 0"), 0644)

	oldReadFile := readFile
	readFile = func(p string) ([]byte, error) {
		if strings.Contains(p, "stat") {
			return os.ReadFile(statPath)
		}
		return nil, errors.New("unexpected")
	}
	defer func() { readFile = oldReadFile }()

	cpuCacheMap = make(map[int]*cpuCache)
	_, _ = GetCPUFromProc(1) // prime
	time.Sleep(120 * time.Millisecond)
	// change stat content to simulate more ticks
	os.WriteFile(statPath, []byte("1 (cmd) S 0 0 0 0 0 0 0 0 0 200 150 0 0 0 0 0 0 0 0 0 0"), 0644)
	v, err := GetCPUFromProc(1)
	if err != nil {
		t.Fatalf("GetCPUFromProc err: %v", err)
	}
	if v <= 0 {
		t.Fatalf("expected cpu usage >0, got %f", v)
	}
}

func TestRemoveCgroup(t *testing.T) {
	tmp := t.TempDir()
	config.CgroupRoot = tmp
	defer func() { config.CgroupRoot = "/sys/fs/cgroup/QubeContainers" }()

	os.MkdirAll(filepath.Join(tmp, "c1"), 0755)
	if err := RemoveCgroup("c1"); err != nil {
		t.Fatalf("RemoveCgroup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "c1")); !os.IsNotExist(err) {
		t.Fatalf("expected removal")
	}
}
