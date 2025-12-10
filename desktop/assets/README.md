# Qube Desktop Icon Placeholder

This directory should contain application icons:

- `icon.png` - Linux icon (512x512 or larger)
- `icon.ico` - Windows icon (256x256)
- `logo.png` - Application logo for the UI

## Creating Icons

You can use the existing Qube logo from `../docs/assets/images/logo.png` or create custom ones.

### Quick Setup

If you have ImageMagick installed:

```bash
# Convert logo to different formats
convert ../docs/assets/images/logo.png -resize 512x512 icon.png
convert ../docs/assets/images/logo.png -resize 256x256 icon.ico

# Copy logo for UI
cp ../docs/assets/images/logo.png logo.png
```

Or manually:
1. Copy your logo to this directory as `logo.png`
2. Create a 512x512 PNG for Linux as `icon.png`
3. Create a 256x256 ICO for Windows as `icon.ico`

The build process will automatically use these icons for the respective platforms.
