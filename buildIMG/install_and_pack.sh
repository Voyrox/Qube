#!/bin/bash

ROOTFS="ubuntu-24.04-server-cloudimg-amd64-root.tar.xz"
MOUNT_DIR="/mnt/rootfs"
LOG_FILE="/mnt/install_log.txt"
OUTPUT_TAR="${PWD}/Ubuntu24_Multi.tar"

mkdir -p $MOUNT_DIR

echo "Extracting root filesystem..."
tar -xJf $ROOTFS -C $MOUNT_DIR

echo "Mounting necessary filesystems..."
mount --bind /dev $MOUNT_DIR/dev
mount --bind /proc $MOUNT_DIR/proc
mount --bind /sys $MOUNT_DIR/sys
mount --bind /run $MOUNT_DIR/run

chroot $MOUNT_DIR /bin/bash <<EOF
# Update package list
apt-get update
apt-get install -y software-properties-common
rm -rf /etc/resolv.conf
echo "nameserver 8.8.8.8" > /etc/resolv.conf

# Install Node.js and npm
if [[ "\$INSTALL_NODE" == "true" ]]; then
    echo "Installing Node.js and npm..."
    apt-get install -y nodejs npm
    echo "Node.js version:" > $LOG_FILE
    node -v >> $LOG_FILE
else
    echo "Skipping Node.js and npm installation."
fi

# Install Rust
if [[ "\$INSTALL_RUST" == "true" ]]; then
    echo "Installing Rust..."
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
    source \$HOME/.cargo/env
    echo "Rust version:" >> $LOG_FILE
    rustc --version >> $LOG_FILE
else
    echo "Skipping Rust installation."
fi

# Install Python 3 and pip3
if [[ "\$INSTALL_PYTHON" == "true" ]]; then
    echo "Installing Python 3 and pip3..."
    apt-get install -y python3 python3-pip
    echo "Python 3 version:" >> $LOG_FILE
    python3 --version >> $LOG_FILE
    echo "pip3 version:" >> $LOG_FILE
    pip3 --version >> $LOG_FILE
else
    echo "Skipping Python 3 and pip3 installation."
fi

# Install Java
if [[ "\$INSTALL_JAVA" == "true" ]]; then
    echo "Installing Java..."
    apt-get install -y openjdk-11-jdk
    echo "Java version:" >> $LOG_FILE
    java -version >> $LOG_FILE
else
    echo "Skipping Java installation."
fi

# Install Go (Golang)
if [[ "\$INSTALL_GOLANG" == "true" ]]; then
    echo "Installing Go..."
    apt-get install -y golang
    echo "Go version:" >> $LOG_FILE
    go version >> $LOG_FILE
else
    echo "Skipping Go installation."
fi
EOF

umount $MOUNT_DIR/dev
umount $MOUNT_DIR/proc
umount $MOUNT_DIR/sys
umount $MOUNT_DIR/run

echo "Creating new root filesystem tarball at $OUTPUT_TAR..."
tar -czf $OUTPUT_TAR -C $MOUNT_DIR .

echo "Removing old rootfs directory..."
sudo rm -rf $MOUNT_DIR

echo "New root filesystem tarball created: $OUTPUT_TAR"
echo "Installation complete. Log file: $LOG_FILE"
