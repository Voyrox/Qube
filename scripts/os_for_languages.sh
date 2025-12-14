#!/bin/bash

set -e  # Exit on any error

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root (use sudo)" 
   exit 1
fi

ROOTFS="ubuntu-24.04-server-cloudimg-amd64-root.tar.xz"
MOUNT_DIR="/mnt/rootfs"
LOG_FILE="/tmp/install_log.txt"

if [[ ! -f "$ROOTFS" ]]; then
    echo "Error: Root filesystem $ROOTFS not found!"
    exit 1
fi

cleanup() {
    echo "Cleaning up..."
    umount "$MOUNT_DIR/dev" 2>/dev/null || true
    umount "$MOUNT_DIR/proc" 2>/dev/null || true
    umount "$MOUNT_DIR/sys" 2>/dev/null || true
    umount "$MOUNT_DIR/run" 2>/dev/null || true
    rm -rf "$MOUNT_DIR"
}

create_rootfs_for_language() {
    LANGUAGE="$1"
    OUTPUT_TAR="${PWD}/Ubuntu24_${LANGUAGE}.tar.gz"

    echo "Creating root filesystem for $LANGUAGE..."

    trap cleanup EXIT

    cleanup

    mkdir -p "$MOUNT_DIR"

    echo "Extracting root filesystem..."
    tar -xJf "$ROOTFS" -C "$MOUNT_DIR"

    echo "Mounting necessary filesystems..."
    mount --bind /dev "$MOUNT_DIR/dev"
    mount --bind /proc "$MOUNT_DIR/proc"
    mount --bind /sys "$MOUNT_DIR/sys"
    mount --bind /run "$MOUNT_DIR/run"

    chroot "$MOUNT_DIR" /bin/bash <<EOF
set -e
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y software-properties-common curl
rm -rf /etc/resolv.conf
echo "nameserver 8.8.8.8" > /etc/resolv.conf

if [[ "$LANGUAGE" == "NODE" ]]; then
    echo "Installing Node.js and npm..."
    curl -fsSL https://deb.nodesource.com/setup_25.x | bash -
    apt-get install -y nodejs
    echo "Node.js version:"
    node -v
    echo "npm version:"
    npm -v
    # Clean npm cache to avoid permission issues
    npm cache clean --force
fi

if [[ "$LANGUAGE" == "RUST" ]]; then
    echo "Installing Rust..."
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
    source \$HOME/.cargo/env
    echo "Rust version:"
    rustc --version
fi

if [[ "$LANGUAGE" == "PYTHON" ]]; then
    echo "Installing Python 3 and pip3..."
    apt-get install -y python3 python3-pip
    echo "Python 3 version:"
    python3 --version
    echo "pip3 version:"
    pip3 --version
fi

if [[ "$LANGUAGE" == "JAVA" ]]; then
    echo "Installing Java..."
    apt-get install -y openjdk-11-jdk
    echo "Java version:"
    java -version 2>&1
fi

if [[ "$LANGUAGE" == "GOLANG" ]]; then
    echo "Installing Go..."
    apt-get install -y golang
    echo "Go version:"
    go version
fi

# Cleanup
apt-get clean
rm -rf /var/lib/apt/lists/*
rm -rf /root/.npm
rm -rf /root/.cache
rm -rf /tmp/*
EOF

    echo "Unmounting filesystems..."
    umount "$MOUNT_DIR/dev"
    umount "$MOUNT_DIR/proc"
    umount "$MOUNT_DIR/sys"
    umount "$MOUNT_DIR/run"

    echo "Setting all files to be owned by root..."
    chown -R 0:0 "$MOUNT_DIR"
    chmod -R u+rwX,go+rX "$MOUNT_DIR"

    echo "Creating new root filesystem tarball at $OUTPUT_TAR..."
    tar --numeric-owner -czf "$OUTPUT_TAR" -C "$MOUNT_DIR" .
    
    if [[ -n "$SUDO_USER" ]]; then
        chown $SUDO_USER:$SUDO_USER "$OUTPUT_TAR"
    fi

    echo "Removing old rootfs directory..."
    rm -rf "$MOUNT_DIR"

    echo "Root filesystem tarball for $LANGUAGE created: $OUTPUT_TAR"
    echo "Size: $(du -h "$OUTPUT_TAR" | cut -f1)"

    trap - EXIT
}

LANGUAGES=("NODE" "RUST" "PYTHON" "JAVA" "GOLANG")

for LANGUAGE in "${LANGUAGES[@]}"; do
    create_rootfs_for_language "$LANGUAGE"
done

echo "All root filesystem tarballs have been created."
