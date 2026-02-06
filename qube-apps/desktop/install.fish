#!/usr/bin/env fish

# Qube Desktop Installation Script
# For Ubuntu, Arch Linux, and other Linux distributions

set -e

echo "🚀 Qube Desktop Installation Script"
echo "===================================="
echo ""

# Check if Node.js is installed
if not command -v node &> /dev/null
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
end

# Check Node.js version
set node_version (node -v | sed 's/v//' | cut -d '.' -f 1)
if test $node_version -lt 16
    echo "❌ Node.js version is too old (need 16+, have $node_version)"
    echo "Please upgrade Node.js"
    exit 1
end

echo "✅ Node.js version: "(node -v)
echo ""

# Navigate to desktop directory
set script_dir (dirname (status -f))
cd $script_dir

echo "📦 Installing dependencies..."
npm install

if test $status -ne 0
    echo "❌ Failed to install dependencies"
    exit 1
end

echo ""
echo "🔨 Building application..."
npm run build:linux

if test $status -ne 0
    echo "❌ Failed to build application"
    exit 1
end

echo ""
echo "✅ Build completed successfully!"
echo ""
echo "📦 Installation packages created:"
echo ""

# List created files
if test -d dist
    echo "Available in ./dist/:"
    ls -lh dist/*.{AppImage,deb,rpm,tar.gz} 2>/dev/null | awk '{print "  - " $9 " (" $5 ")"}'
else
    echo "⚠️  Dist directory not found"
    exit 1
end

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
