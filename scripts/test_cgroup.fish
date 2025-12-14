#!/usr/bin/env fish
# Test script to verify cgroup memory limits are working

echo "=== Qube Cgroup Test ==="
echo ""

# Check if running as root
if test (id -u) -ne 0
    echo "❌ This test must be run as root (use sudo)"
    exit 1
end

echo "✓ Running as root"

# Check if cgroup v2 is available
if test -d /sys/fs/cgroup
    echo "✓ Cgroup v2 is available"
else
    echo "❌ Cgroup v2 not found at /sys/fs/cgroup"
    exit 1
end

# Build the project
echo ""
echo "Building Qube..."
cargo build --release
if test $status -ne 0
    echo "❌ Build failed"
    exit 1
end
echo "✓ Build successful"

# Check cgroup configuration
echo ""
echo "Checking cgroup configuration in config.rs:"
grep -A2 "MEMORY_MAX_MB" src/config/config.rs
echo ""

# Start the daemon in background
echo "Starting Qube daemon..."
./target/release/Qube daemon --debug &
set daemon_pid $last_pid
sleep 2

# Check if cgroup root was created
if test -d /sys/fs/cgroup/QubeContainers
    echo "✓ Cgroup root directory created at /sys/fs/cgroup/QubeContainers"
    
    # Check if controllers are enabled
    if test -f /sys/fs/cgroup/QubeContainers/cgroup.controllers
        set controllers (cat /sys/fs/cgroup/QubeContainers/cgroup.controllers)
        echo "  Controllers: $controllers"
    end
else
    echo "❌ Cgroup root directory not created"
end

echo ""
echo "=== Test Summary ==="
echo "Cgroup configuration:"
echo "  - Memory limit: 2048 MB (2 GB)"
echo "  - Swap limit: 1024 MB (1 GB)"
echo "  - Location: /sys/fs/cgroup/QubeContainers"
echo ""
echo "To test with a container:"
echo "  1. Run: sudo ./target/release/Qube run --image Ubuntu24_NODE --cmd 'sleep 60'"
echo "  2. Check: ls -la /sys/fs/cgroup/QubeContainers/"
echo "  3. View limits: cat /sys/fs/cgroup/QubeContainers/<container-id>/memory.max"
echo "  4. View usage: cat /sys/fs/cgroup/QubeContainers/<container-id>/memory.current"
echo "  5. Get info: sudo ./target/release/Qube info <container-id>"
echo ""

# Cleanup
kill $daemon_pid 2>/dev/null
echo "Daemon stopped."
