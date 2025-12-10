# Quick Start Guide for Qube Desktop

## 🚀 Quick Setup & Run

```bash
cd desktop
chmod +x install.sh  # or install.fish if you use fish shell
./install.sh         # or ./install.fish
```

Or manually:

```bash
cd desktop
npm install
npm start
```

## 📦 Build Distribution Packages

### For Your Current Platform
```bash
npm run build
```

### For Linux Only
```bash
npm run build:linux
```

### For Windows Only
```bash
npm run build:win
```

### For All Platforms
```bash
npm run build:all
```

## 🎯 Development Mode

Run with DevTools open for debugging:

```bash
npm run dev
```

## 📋 Next Steps

1. Make sure the Qube daemon (`qubed`) is running
2. Verify API is accessible at http://127.0.0.1:3030
3. Launch the desktop app
4. Enjoy managing your containers!

## 🐛 Troubleshooting

See the main README.md for troubleshooting tips.
