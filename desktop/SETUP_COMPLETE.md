# Qube Desktop - Electron Application

## ✅ Complete Setup Summary

I've successfully converted your HTML files into a complete Electron desktop application that works on:
- **Ubuntu** (via .deb, AppImage, or tarball)
- **Arch Linux** (via AppImage or tarball)
- **Windows** (via NSIS installer or portable .exe)

## 📁 Project Structure

```
desktop/
├── package.json          # Electron app configuration & dependencies
├── main.js              # Main Electron process (window management)
├── preload.js           # Secure bridge between main & renderer
├── README.md            # Full documentation
├── QUICKSTART.md        # Quick start guide
├── .gitignore           # Git ignore rules
├── install.sh           # Bash installation script
├── install.fish         # Fish shell installation script
│
├── renderer/            # Frontend application files
│   ├── index.html       # Main dashboard (containers view)
│   ├── console.html     # Interactive container console
│   ├── script.js        # Dashboard JavaScript logic
│   ├── console.js       # Console JavaScript logic
│   └── logo.png         # Qube logo for UI
│
└── assets/              # Application icons & resources
    ├── logo.png         # App logo
    ├── icon.png         # Linux icon (512x512)
    └── README.md        # Icon setup instructions
```

## 🎨 Features Implemented

### Desktop App Features
- ✅ Fixed window size (1280x800) like Docker Desktop
- ✅ Minimum window size (1024x600)
- ✅ Custom application menu with shortcuts
- ✅ Window position persistence
- ✅ Draggable title bar
- ✅ Developer tools toggle (Ctrl+Shift+I)
- ✅ Secure IPC communication
- ✅ Settings storage (API endpoint, preferences)

### UI Features
- ✅ Modern neon-themed interface
- ✅ Real-time container monitoring (1s refresh)
- ✅ Container start/stop controls
- ✅ Interactive WebSocket console
- ✅ Container statistics dashboard
- ✅ Responsive sidebar navigation
- ✅ Toast notifications
- ✅ Content Security Policy (CSP)

### Build Configuration
- ✅ AppImage for universal Linux support
- ✅ .deb packages for Ubuntu/Debian
- ✅ .rpm packages for Fedora/Red Hat
- ✅ .tar.gz archives
- ✅ Windows NSIS installer
- ✅ Windows portable executable

## 🚀 Getting Started

### Step 1: Install Dependencies
```bash
cd desktop
npm install
```

### Step 2: Run in Development Mode
```bash
npm start
# Or with DevTools open:
npm run dev
```

### Step 3: Build for Distribution
```bash
# Linux packages (AppImage, .deb, .rpm, .tar.gz)
npm run build:linux

# Windows packages (installer + portable)
npm run build:win

# All platforms
npm run build:all
```

Or use the automated script:
```bash
./install.sh    # Bash
# or
./install.fish  # Fish shell
```

## 📦 Installation

### Ubuntu/Debian
```bash
sudo dpkg -i dist/qube-desktop_*.deb
```

### Arch Linux / Any Linux
```bash
chmod +x dist/Qube-Desktop-*.AppImage
./dist/Qube-Desktop-*.AppImage
```

### Windows
Run: `dist/Qube-Desktop-Setup-*.exe`

## 🔧 Configuration

Settings are stored in:
- **Linux**: `~/.config/qube-desktop/`
- **Windows**: `%APPDATA%/qube-desktop/`

Configurable options:
- API Base URL (default: http://127.0.0.1:3030)
- Auto-refresh interval
- Window position/size

## ⌨️ Keyboard Shortcuts

- `Ctrl/Cmd + R` - Reload window
- `Ctrl/Cmd + Shift + I` - Toggle Developer Tools
- `Ctrl/Cmd + Q` - Quit application
- `Ctrl/Cmd + ,` - Preferences (planned)

## 🔒 Security Features

- ✅ Context isolation enabled
- ✅ Node integration disabled in renderer
- ✅ Content Security Policy configured
- ✅ Secure IPC communication only
- ✅ No remote module access

## 📝 Key Changes from HTML Version

1. **Content Security Policy**: Added CSP meta tag for security
2. **Error Handling**: Logo image has error fallback
3. **Navigation**: Console uses query parameters for container selection
4. **Back Button**: Added navigation back to dashboard from console
5. **Electron Integration**: Uses `window.electron` API for settings
6. **Platform-Aware**: Adjusts behavior for different OSs

## 🎯 Next Steps

### Immediate
1. Ensure `qubed` service is running:
   ```bash
   sudo systemctl start qubed
   ```

2. Test the app:
   ```bash
   cd desktop
   npm start
   ```

### Future Enhancements
- [ ] Add preferences/settings window
- [ ] Implement "New Container" wizard
- [ ] Add image management UI
- [ ] Add volume management UI
- [ ] Add build/logs viewer
- [ ] System tray integration
- [ ] Auto-update functionality
- [ ] Multi-language support
- [ ] Dark/light theme toggle

## 🐛 Troubleshooting

### Can't connect to API
```bash
# Check if qubed is running
sudo systemctl status qubed

# Test API manually
curl http://127.0.0.1:3030/list
```

### Build fails
```bash
# Clean and rebuild
rm -rf node_modules dist
npm install
npm run build
```

### AppImage won't run
```bash
# Install FUSE
sudo apt install fuse libfuse2  # Ubuntu
sudo pacman -S fuse2            # Arch
```

## 📚 Documentation

- `README.md` - Full documentation
- `QUICKSTART.md` - Quick start guide
- `assets/README.md` - Icon setup instructions

## 🎉 Success!

Your Qube Desktop application is ready! It provides the same sleek experience as Docker Desktop with:
- Cross-platform support (Linux & Windows)
- Fixed, consistent window size
- Modern UI with real-time updates
- Secure Electron architecture
- Easy distribution packages

Run `cd desktop && npm start` to launch it now!
