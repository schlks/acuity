#!/usr/bin/env bash
set -e

# Build standalone AppImage for Linux
ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT_DIR"

echo "==> Building Acuity binary with Wails..."
wails build -clean -tags webkit2_41

APPDIR="$ROOT_DIR/build/AppDir"
rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin"
mkdir -p "$APPDIR/usr/share/applications"
mkdir -p "$APPDIR/usr/share/icons/hicolor/scalable/apps"

# Copy files
cp "$ROOT_DIR/build/bin/acuity" "$APPDIR/usr/bin/acuity"
cp "$ROOT_DIR/install/linux/acuity.desktop" "$APPDIR/acuity.desktop"
cp "$ROOT_DIR/install/linux/acuity.desktop" "$APPDIR/usr/share/applications/acuity.desktop"
cp "$ROOT_DIR/ui/static/logo.svg" "$APPDIR/acuity.svg"
cp "$ROOT_DIR/ui/static/logo.svg" "$APPDIR/usr/share/icons/hicolor/scalable/apps/acuity.svg"

# Create AppRun
cat << 'EOF' > "$APPDIR/AppRun"
#!/bin/sh
HERE="$(dirname "$(readlink -f "${0}")")"
exec "${HERE}/usr/bin/acuity" "$@"
EOF
chmod +x "$APPDIR/AppRun"

# Download appimagetool if not found
if ! command -v appimagetool &>/dev/null; then
    echo "==> Downloading appimagetool..."
    wget -q -O /tmp/appimagetool "https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage"
    chmod +x /tmp/appimagetool
    TOOL="/tmp/appimagetool"
else
    TOOL="appimagetool"
fi

echo "==> Generating AppImage..."
ARCH=x86_64 "$TOOL" "$APPDIR" "$ROOT_DIR/build/bin/Acuity-x86_64.AppImage"
echo "==> Success: AppImage created at build/bin/Acuity-x86_64.AppImage"
