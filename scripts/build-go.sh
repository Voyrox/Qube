#!/bin/bash

# Build script for Qube (Bash)

BINARY_NAME=qube
INSTALL_PATH=/usr/local/bin

echo "🔨 Building Qube..."
go build -ldflags="-s -w" -o "$BINARY_NAME" ./cmd/qube

if [ $? -eq 0 ]; then
    echo "✓ Build successful: ./$BINARY_NAME"
    
    if [ "$1" == "--install" ]; then
        echo "📦 Installing Qube..."
        sudo rm -f "$INSTALL_PATH/$BINARY_NAME"
        sudo cp "$BINARY_NAME" "$INSTALL_PATH/$BINARY_NAME"
        sudo chmod +x "$INSTALL_PATH/$BINARY_NAME"
        echo "✓ Installed to $INSTALL_PATH/$BINARY_NAME"
        
        if [ -f qubed.service ]; then
            sudo cp qubed.service /etc/systemd/system/qubed.service
            sudo systemctl daemon-reload
            echo "✓ Service file installed"
            echo "  Run 'sudo systemctl start qubed' to start the daemon"
        fi
    fi
else
    echo "❌ Build failed"
    exit 1
fi
