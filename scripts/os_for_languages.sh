#!/bin/bash

ROOTFS="ubuntu-24.04-server-cloudimg-amd64-root.tar.xz"
MOUNT_DIR="/mnt/rootfs"
LOG_FILE="/mnt/install_log.txt"

create_rootfs_for_language() {
    LANGUAGE="$1"
    OUTPUT_TAR="${PWD}/Ubuntu24_${LANGUAGE}.tar"

    echo "Creating root filesystem for $LANGUAGE..."

    sudo mkdir -p "$MOUNT_DIR"

    echo "Extracting root filesystem..."
    tar -xJf "$ROOTFS" -C "$MOUNT_DIR"

    echo "Mounting necessary filesystems..."
    mount --bind /dev "$MOUNT_DIR/dev"
    mount --bind /proc "$MOUNT_DIR/proc"
    mount --bind /sys "$MOUNT_DIR/sys"
    mount --bind /run "$MOUNT_DIR/run"

    chroot "$MOUNT_DIR" /bin/bash <<EOF
apt-get update
apt-get install -y software-properties-common
rm -rf /etc/resolv.conf
echo "nameserver 8.8.8.8" > /etc/resolv.conf

if [[ "$LANGUAGE" == "NODE" ]]; then
    echo "Installing Node.js and npm..." >> $LOG_FILE
    curl -fsSL https://deb.nodesource.com/setup_23.x | bash -
    apt-get install -y nodejs
    echo "Node.js version:" >> $LOG_FILE
    node -v >> $LOG_FILE
    echo "npm version:" >> $LOG_FILE
    npm -v >> $LOG_FILE
fi

if [[ "$LANGUAGE" == "RUST" ]]; then
    echo "Installing Rust..." >> $LOG_FILE
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
    source \$HOME/.cargo/env
    echo "Rust version:" >> $LOG_FILE
    rustc --version >> $LOG_FILE
fi

if [[ "$LANGUAGE" == "PYTHON" ]]; then
    echo "Installing Python 3 and pip3..." >> $LOG_FILE
    apt-get install -y python3 python3-pip
    echo "Python 3 version:" >> $LOG_FILE
    python3 --version >> $LOG_FILE
    echo "pip3 version:" >> $LOG_FILE
    pip3 --version >> $LOG_FILE
fi

if [[ "$LANGUAGE" == "JAVA" ]]; then
    echo "Installing Java..." >> $LOG_FILE
    apt-get install -y openjdk-11-jdk
    echo "Java version:" >> $LOG_FILE
    java -version >> $LOG_FILE
fi

if [[ "$LANGUAGE" == "GOLANG" ]]; then
    echo "Installing Go..." >> $LOG_FILE
    apt-get install -y golang
    echo "Go version:" >> $LOG_FILE
    go version >> $LOG_FILE
fi
EOF

    umount "$MOUNT_DIR/dev"
    umount "$MOUNT_DIR/proc"
    umount "$MOUNT_DIR/sys"
    umount "$MOUNT_DIR/run"

    echo "Creating new root filesystem tarball at $OUTPUT_TAR..."
    tar -czf "$OUTPUT_TAR" -C "$MOUNT_DIR" .

    echo "Removing old rootfs directory..."
    sudo rm -rf "$MOUNT_DIR"

    echo "Root filesystem tarball for $LANGUAGE created: $OUTPUT_TAR"
    echo "Installation complete. Log file: $LOG_FILE"
}

LANGUAGES=("NODE" "RUST" "PYTHON" "JAVA" "GOLANG")

for LANGUAGE in "${LANGUAGES[@]}"; do
    create_rootfs_for_language "$LANGUAGE"
done

echo "All root filesystem tarballs have been created."
