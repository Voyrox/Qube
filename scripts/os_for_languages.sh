#!/bin/bash

set -e  # Exit on any error

ROOTFS="ubuntu-24.04-server-cloudimg-amd64-root.tar.xz"
MOUNT_DIR="/mnt/rootfs"
LOG_FILE="/tmp/install_log.txt"

if [[ ! -f "$ROOTFS" ]]; then
    echo "Error: Root filesystem $ROOTFS not found!"
    exit 1
fi

cleanup() {
    echo "Cleaning up..."
    sudo umount "$MOUNT_DIR/dev" 2>/dev/null || true
    sudo umount "$MOUNT_DIR/proc" 2>/dev/null || true
    sudo umount "$MOUNT_DIR/sys" 2>/dev/null || true
    sudo umount "$MOUNT_DIR/run" 2>/dev/null || true
    sudo rm -rf "$MOUNT_DIR"
}

create_rootfs_for_language() {
    LANGUAGE="$1"
    OUTPUT_TAR="${PWD}/Ubuntu24_${LANGUAGE}.tar.gz"

    echo "Creating root filesystem for $LANGUAGE..."

    trap cleanup EXIT

    cleanup

    sudo mkdir -p "$MOUNT_DIR"

    echo "Extracting root filesystem..."
    sudo tar -xJf "$ROOTFS" -C "$MOUNT_DIR"

    echo "Mounting necessary filesystems..."
    sudo mount --bind /dev "$MOUNT_DIR/dev"
    sudo mount --bind /proc "$MOUNT_DIR/proc"
    sudo mount --bind /sys "$MOUNT_DIR/sys"
    sudo mount --bind /run "$MOUNT_DIR/run"

    sudo chroot "$MOUNT_DIR" /bin/bash <<EOF
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
EOF

    echo "Unmounting filesystems..."
    sudo umount "$MOUNT_DIR/dev"
    sudo umount "$MOUNT_DIR/proc"
    sudo umount "$MOUNT_DIR/sys"
    sudo umount "$MOUNT_DIR/run"

    echo "Creating new root filesystem tarball at $OUTPUT_TAR..."
    sudo tar -czf "$OUTPUT_TAR" -C "$MOUNT_DIR" .
    sudo chown $USER:$USER "$OUTPUT_TAR"

    echo "Removing old rootfs directory..."
    sudo rm -rf "$MOUNT_DIR"

    echo "Root filesystem tarball for $LANGUAGE created: $OUTPUT_TAR"
    echo "Size: $(du -h "$OUTPUT_TAR" | cut -f1)"

    trap - EXIT
}

LANGUAGES=("NODE" "RUST" "PYTHON" "JAVA" "GOLANG")

for LANGUAGE in "${LANGUAGES[@]}"; do
    create_rootfs_for_language "$LANGUAGE"
done

echo "All root filesystem tarballs have been created."
