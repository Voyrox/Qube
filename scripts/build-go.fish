#!/usr/bin/env fish

# Build script for Qube (Fish shell)

set BINARY_NAME qube
set INSTALL_PATH /usr/local/bin

echo "🔨 Building Qube..."
go build -ldflags="-s -w" -o $BINARY_NAME ./cmd/qube

if test $status -eq 0
    echo "✓ Build successful: ./$BINARY_NAME"
    
    if test (count $argv) -gt 0 -a "$argv[1]" = "--install"
        echo "📦 Installing Qube..."
        sudo rm -f $INSTALL_PATH/$BINARY_NAME
        sudo cp $BINARY_NAME $INSTALL_PATH/$BINARY_NAME
        sudo chmod +x $INSTALL_PATH/$BINARY_NAME
        echo "✓ Installed to $INSTALL_PATH/$BINARY_NAME"
        
        if test -f qubed.service
            sudo cp qubed.service /etc/systemd/system/qubed.service
            sudo systemctl daemon-reload
            echo "✓ Service file installed"
            echo "  Run 'sudo systemctl start qubed' to start the daemon"
        end
    end
else
    echo "❌ Build failed"
    exit 1
end
