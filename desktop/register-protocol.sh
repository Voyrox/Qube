#!/bin/bash
# Register the qube:// protocol for Linux

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Registering Qube protocol handler...${NC}"

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

DESKTOP_FILE="$HOME/.local/share/applications/qube-desktop.desktop"
mkdir -p "$HOME/.local/share/applications"


# Try to find AppImage first
APPIMAGE_PATH="$SCRIPT_DIR/dist/Qube Desktop-1.0.0.AppImage"
if [ -f "$APPIMAGE_PATH" ]; then
  # Use double quotes for Exec, escape as needed
  EXEC_CMD="\"$APPIMAGE_PATH\" %u"
  ICON_PATH="$SCRIPT_DIR/assets/icon.png"
  echo -e "${GREEN}Using AppImage: $APPIMAGE_PATH${NC}"
  cat > "$DESKTOP_FILE" << EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=Qube Desktop
Comment=Container Management GUI
Exec=$EXEC_CMD
Icon=$ICON_PATH
Terminal=false
Categories=Development;
MimeType=x-scheme-handler/qube;
StartupNotify=true
EOF
else
  ELECTRON_PATH=$(which electron || echo "electron")
  APP_PATH="$SCRIPT_DIR/main.js"
  EXEC_CMD="$ELECTRON_PATH \"$APP_PATH\" %u"
  ICON_PATH="$SCRIPT_DIR/assets/icon.png"
  echo -e "${YELLOW}AppImage not found, falling back to electron: $ELECTRON_PATH $APP_PATH${NC}"
  cat > "$DESKTOP_FILE" << EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=Qube Desktop
Comment=Container Management GUI
Exec=$EXEC_CMD
Icon=$ICON_PATH
Terminal=false
Categories=Development;
MimeType=x-scheme-handler/qube;
StartupNotify=true
EOF
fi

echo -e "${GREEN}Created desktop file at: $DESKTOP_FILE${NC}"

if command -v update-desktop-database &> /dev/null; then
  update-desktop-database "$HOME/.local/share/applications"
  echo -e "${GREEN}Updated desktop database${NC}"
fi

xdg-mime default qube-desktop.desktop x-scheme-handler/qube

echo -e "${GREEN}Protocol registered!${NC}"
echo -e "${YELLOW}You can now test it by running: xdg-open 'qube://container/mycontainer'${NC}"
echo -e "${YELLOW}Or create a link on your web page: <a href='qube://container/mycontainer'>Open in App</a>${NC}"
