# Cgroup Resource Limits in Qube

## Overview

Qube now properly implements cgroup v2 resource limits for containers. This ensures that containers cannot consume more than their allocated resources, preventing resource exhaustion on the host system.

## Configuration

Resource limits are configured in `src/config/config.rs`:

```rust
/// Maximum memory limit for containers.
pub const MEMORY_MAX_MB: u64 = 2048; // 2 GB
pub const MEMORY_SWAP_MAX_MB: u64 = 1024; // 1 GB

/// Cgroup root for container isolation.
pub const CGROUP_ROOT: &str = "/sys/fs/cgroup/QubeContainers";
```

## How It Works

### 1. Daemon Startup
When the Qube daemon starts, it:
- Creates `/sys/fs/cgroup/QubeContainers` if it doesn't exist
- Enables `memory` and `cpu` controllers
- Sets proper permissions (755)

### 2. Container Creation
When a container is created:
- **Parent Process** (before fork):
  - Creates a cgroup directory: `/sys/fs/cgroup/QubeContainers/<container-id>`
  - Sets `memory.max` to 2048 MB (2 GB)
  - Sets `memory.swap.max` to 1024 MB (1 GB)
  
- **After Fork**:
  - Adds the container's PID to `cgroup.procs`
  - The container process and all its children are now limited

### 3. Container Deletion
When a container is stopped or deleted:
- The cgroup directory is automatically cleaned up
- Resources are freed

## Verifying Cgroup Limits

### Check if cgroups are enabled:
```bash
# Check cgroup root exists
ls -la /sys/fs/cgroup/QubeContainers/

# Check controllers
cat /sys/fs/cgroup/QubeContainers/cgroup.controllers
```

### Check a specific container:
```bash
# List all container cgroups
ls -la /sys/fs/cgroup/QubeContainers/

# View memory limit
cat /sys/fs/cgroup/QubeContainers/<container-id>/memory.max
# Should show: 2147483648 (2048 MB in bytes)

# View current memory usage
cat /sys/fs/cgroup/QubeContainers/<container-id>/memory.current

# View memory stats
cat /sys/fs/cgroup/QubeContainers/<container-id>/memory.stat
```

### Using Qube info command:
```bash
sudo qube info <container-id>
```

This will show:
- Container details
- Current memory usage
- Memory limit
- Percentage used

## Testing Memory Limits

### Test 1: Run a memory-intensive container
```bash
# Start daemon
sudo qube daemon --debug

# In another terminal, create a container that tries to allocate 3GB (more than limit)
sudo qube run --image Ubuntu24_NODE --cmd "sh -c 'dd if=/dev/zero of=/dev/null bs=1M count=3000'"

# Watch memory usage
watch -n 1 'cat /sys/fs/cgroup/QubeContainers/*/memory.current'
```

The container should be killed by the OOM killer when it tries to exceed 2GB.

### Test 2: Monitor with info command
```bash
# Start a long-running container
sudo qube run --image Ubuntu24_NODE --cmd "sleep 300"

# Check its memory usage
sudo qube info <container-id>
```

## Architecture Changes

### Before (Broken):
- Cgroup setup was called in the **child process**
- Child process didn't have permissions to create cgroup directories
- Memory limits were ignored
- No cleanup on container deletion

### After (Working):
- Cgroup setup happens in the **parent process** (as root)
- Parent creates cgroup directory before forking
- Parent adds child PID to cgroup after fork
- Proper cleanup on container deletion
- Memory usage stats available via `qube info`

## Code Structure

```
src/core/cgroup.rs
├── init_cgroup_root()              # Initialize /sys/fs/cgroup/QubeContainers
├── setup_cgroup_for_container()    # Create per-container cgroup
├── add_process_to_cgroup()         # Add PID to cgroup
├── cleanup_cgroup()                # Remove cgroup on deletion
└── get_memory_stats()              # Get current memory usage

src/core/container/runtime.rs
└── run_container()
    ├── setup_cgroup_for_container() # Called BEFORE fork (as root)
    └── add_process_to_cgroup()      # Called AFTER fork (adds child PID)

src/daemon/daemon.rs
└── start_daemon()
    └── init_cgroup_root()          # Called at daemon startup
```

## Troubleshooting

### "Warning: Failed to initialize cgroup root"
- **Cause**: Running without root permissions
- **Solution**: Run daemon with `sudo`

### "Warning: Failed to enable cgroup controllers"
- **Cause**: Cgroup v2 not available or controllers already in use
- **Solution**: Check `/sys/fs/cgroup/cgroup.controllers`

### Memory limits not enforced
- **Cause**: Cgroup v1 system or hybrid mode
- **Solution**: Ensure cgroup v2 is enabled:
  ```bash
  # Check current mode
  mount | grep cgroup
  
  # Should show: cgroup2 on /sys/fs/cgroup type cgroup2
  ```

### Container not in cgroup
- **Cause**: Cgroup setup failed silently
- **Solution**: Run daemon with `--debug` flag to see warnings

## Future Enhancements

Potential improvements:
- [ ] CPU limits (cpu.max)
- [ ] I/O limits (io.max)
- [ ] PID limits (pids.max)
- [ ] Network bandwidth limits
- [ ] Per-container configurable limits
- [ ] Cgroup metrics in API/dashboard
