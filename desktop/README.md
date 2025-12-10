# Qube Desktop

A modern, cross-platform desktop application for managing Qube containers. Built with Electron for Ubuntu, Arch Linux, and Windows.

![Qube Desktop](screenshot.png)

## Features

- 🎨 Modern, sleek UI similar to Docker Desktop
- 🚀 Cross-platform support (Linux & Windows)
- 📊 Real-time container monitoring
- 🔌 Interactive container console
- 🎯 Fixed window size for consistent experience
- 🔒 Secure IPC communication
- ⚡ Live updates every second

## Requirements

- Node.js 16+ and npm
- Qube daemon running (`qubed` service)
- API server on `http://127.0.0.1:3030`

## Installation

### From Source

```bash
cd desktop
npm install
npm start
```

### Building for Distribution

#### Linux (Ubuntu/Arch)
```bash
npm run build:linux
```

This creates:
- `dist/Qube-Desktop-*.AppImage` - Universal Linux package
- `dist/qube-desktop_*.deb` - Debian/Ubuntu package
- `dist/qube-desktop-*.rpm` - Fedora/Red Hat package
- `dist/qube-desktop-*.tar.gz` - Tarball

#### Windows
```bash
npm run build:win
```

This creates:
- `dist/Qube-Desktop-Setup-*.exe` - NSIS installer
- `dist/Qube-Desktop-*.exe` - Portable executable

#### All Platforms
```bash
npm run build:all
```

## Development

```bash
# Run in development mode with DevTools
npm run dev

# Package without building installers (faster)
npm run pack
```

## Installation Instructions

### Ubuntu/Debian
```bash
sudo dpkg -i dist/qube-desktop_*.deb
# Or use the AppImage
chmod +x dist/Qube-Desktop-*.AppImage
./dist/Qube-Desktop-*.AppImage
```

### Arch Linux
```bash
# Install from AUR or use AppImage
yay -S qube-desktop  # If published to AUR
# Or
chmod +x dist/Qube-Desktop-*.AppImage
./dist/Qube-Desktop-*.AppImage
```

### Windows
Run the installer:
```
dist/Qube-Desktop-Setup-*.exe
```

Or use the portable version:
```
dist/Qube-Desktop-*.exe
```

## Configuration

The app stores settings in:
- **Linux**: `~/.config/qube-desktop/`
- **Windows**: `%APPDATA%/qube-desktop/`

### Settings

- **API Base URL**: Configure the Qube API endpoint (default: `http://127.0.0.1:3030`)
- **Auto-refresh**: Enable/disable automatic container list updates
- **Refresh interval**: Set update frequency (default: 1000ms)

## Architecture

```
desktop/
├── main.js           # Main Electron process
├── preload.js        # Secure preload script
├── package.json      # Dependencies & build config
├── renderer/         # Frontend files
│   ├── index.html    # Main dashboard
│   ├── console.html  # Container console
│   ├── script.js     # Dashboard logic
│   └── console.js    # Console logic
└── assets/           # Icons & resources
```

## Security

- Content Security Policy (CSP) enabled
- Context isolation enabled
- Node integration disabled in renderer
- Secure IPC communication only

## Keyboard Shortcuts

- `Ctrl/Cmd + R` - Reload window
- `Ctrl/Cmd + Shift + I` - Toggle DevTools
- `Ctrl/Cmd + ,` - Preferences (coming soon)
- `Ctrl/Cmd + Q` - Quit application

## Troubleshooting

### Cannot connect to Qube API
1. Ensure `qubed` service is running:
   ```bash
   sudo systemctl status qubed
   ```
2. Check API is accessible:
   ```bash
   curl http://127.0.0.1:3030/list
   ```

### AppImage won't run on Linux
```bash
# Make it executable
chmod +x Qube-Desktop-*.AppImage

# Install FUSE if needed
sudo apt install fuse libfuse2  # Ubuntu/Debian
sudo pacman -S fuse2            # Arch Linux
```

### Build fails
```bash
# Clear cache and reinstall
rm -rf node_modules dist
npm install
npm run build
```

## Contributing

See the main [Qube repository](https://github.com/Voyrox/Qube) for contribution guidelines.

## License

Same as Qube - see LICENSE in the root directory.

## Credits

Built with:
- [Electron](https://www.electronjs.org/)
- [electron-builder](https://www.electron.build/)
- [electron-store](https://github.com/sindresorhus/electron-store)
- Font Awesome icons
- Space Grotesk font
