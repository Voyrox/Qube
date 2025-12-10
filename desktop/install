#!/usr/bin/env bash

# Qube Desktop Installation Script (Bash version)
# For Ubuntu, Arch Linux, and other Linux distributions

set -e

echo "🚀 Qube Desktop Installation Script"
echo "===================================="
echo ""

# Check if Node.js is installed
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not installed!"
    echo "Please install Node.js 16+ first:"
    echo ""
    echo "Ubuntu/Debian:"
    echo "  curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -"
    echo "  sudo apt-get install -y nodejs"
    echo ""
    echo "Arch Linux:"
    echo "  sudo pacman -S nodejs npm"
    exit 1
fi

# Check Node.js version
node_version=$(node -v | sed 's/v//' | cut -d '.' -f 1)
if [ "$node_version" -lt 16 ]; then
    echo "❌ Node.js version is too old (need 16+, have $node_version)"
    echo "Please upgrade Node.js"
    exit 1
fi

echo "✅ Node.js version: $(node -v)"
echo ""

# Navigate to desktop directory
script_dir=$(dirname "$0")
cd "$script_dir"

echo "📦 Installing dependencies..."
npm install

if [ $? -ne 0 ]; then
    echo "❌ Failed to install dependencies"
    exit 1
fi

echo ""
echo "🔨 Building application..."
npm run build:linux

if [ $? -ne 0 ]; then
    echo "❌ Failed to build application"
    exit 1
fi

echo ""
echo "✅ Build completed successfully!"
echo ""
echo "📦 Installation packages created:"
echo ""

# List created files
if [ -d dist ]; then
    echo "Available in ./dist/:"
    ls -lh dist/*.{AppImage,deb,rpm,tar.gz} 2>/dev/null | awk '{print "  - " $9 " (" $5 ")"}'
else
    echo "⚠️  Dist directory not found"
    exit 1
fi

echo ""
echo "📝 Installation Instructions:"
echo ""
echo "Ubuntu/Debian:"
echo "  sudo dpkg -i dist/qube-desktop_*.deb"
echo ""
echo "Arch Linux (or any distro with AppImage):"
echo "  chmod +x dist/Qube-Desktop-*.AppImage"
echo "  ./dist/Qube-Desktop-*.AppImage"
echo ""
echo "Or install from tarball:"
echo "  tar -xzf dist/qube-desktop-*.tar.gz -C /opt/"
echo ""
echo "✨ Done! Qube Desktop is ready to install."
