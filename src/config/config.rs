/// Base URL for image downloads.
pub const BASE_URL: &str = "https://files.ewenmacculloch.com";

/// Base directory where container files are stored.
pub const QUBE_CONTAINERS_BASE: &str = "/var/tmp/Qube_containers";

/// Cgroup root for container isolation.
pub const CGROUP_ROOT: &str = "/sys/fs/cgroup/QubeContainers";

/// Maximum memory limit for containers.
pub const MEMORY_MAX_MB: u64 = 2048; // 2 GB
pub const MEMORY_SWAP_MAX_MB: u64 = 1024; // 1 GB

/// Directory used for tracking container states.
pub const TRACKING_DIR: &str = "/var/lib/Qube";
/// File to store the list of containers.
pub const CONTAINER_LIST_FILE: &str = "/var/lib/Qube/containers.txt";