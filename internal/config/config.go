package config

const (
	BaseURL = "https://files.ewenmacculloch.com"

	QubeContainersBase = "/var/tmp/Qube_containers"

	CgroupRoot = "/sys/fs/cgroup/QubeContainers"

	MemoryMaxMB = 2048

	MemorySwapMaxMB = 1024

	CPUQuotaUS = 200000

	CPUPeriodUS = 100000

	TrackingDir = "/var/lib/Qube"

	ContainerListFile = "/var/lib/Qube/containers.txt"
)
