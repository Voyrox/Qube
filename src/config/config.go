package config

import (
	"os"
	"path/filepath"
)

const (
	BaseURL = "https://hub.qubecloud.org"

	MemoryMaxMB     = 2048
	MemorySwapMaxMB = 1024
	CPUQuotaUS      = 200000
	CPUPeriodUS     = 100000
)

var (
	QubeContainersBase = "/var/tmp/Qube_containers"
	CgroupRoot         = "/sys/fs/cgroup/QubeContainers"
	TrackingDir        = "/var/lib/Qube"
	ContainerListFile  = "/var/lib/Qube/containers.txt"
)

func SetPathsForTests(tempRoot string) (cleanup func()) {
	origQubeBase := QubeContainersBase
	origCgroupRoot := CgroupRoot
	origTrackingDir := TrackingDir
	origContainerListFile := ContainerListFile

	QubeContainersBase = filepath.Join(tempRoot, "containers")
	CgroupRoot = filepath.Join(tempRoot, "cgroup")
	TrackingDir = filepath.Join(tempRoot, "tracking")
	ContainerListFile = filepath.Join(tempRoot, "tracking", "containers.txt")

	_ = os.MkdirAll(TrackingDir, 0755)

	return func() {
		QubeContainersBase = origQubeBase
		CgroupRoot = origCgroupRoot
		TrackingDir = origTrackingDir
		ContainerListFile = origContainerListFile
	}
}
