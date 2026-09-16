#!/usr/bin/env bash
set -e

# Build standalone, portable AppImage for Linux
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
cp "$ROOT_DIR/install/linux/acuity.desktop" "$APPDIR/usr/share/applications/acuity.desktop"
cp "$ROOT_DIR/ui/static/logo.svg" "$APPDIR/usr/share/icons/hicolor/scalable/apps/acuity.svg"

TOOLDIR="$ROOT_DIR/build/tools"
mkdir -p "$TOOLDIR"

# Download linuxdeploy + GTK plugin if not already present
if [ ! -x "$TOOLDIR/linuxdeploy" ]; then
    echo "==> Downloading linuxdeploy..."
    wget -q -O "$TOOLDIR/linuxdeploy" "https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage"
    chmod +x "$TOOLDIR/linuxdeploy"
fi
if [ ! -x "$TOOLDIR/linuxdeploy-plugin-gtk.sh" ]; then
    echo "==> Downloading linuxdeploy GTK plugin..."
    wget -q -O "$TOOLDIR/linuxdeploy-plugin-gtk.sh" "https://raw.githubusercontent.com/linuxdeploy/linuxdeploy-plugin-gtk/master/linuxdeploy-plugin-gtk.sh"
    chmod +x "$TOOLDIR/linuxdeploy-plugin-gtk.sh"
fi

echo "==> Bundling runtime dependencies (libvips, WebKitGTK, GTK modules)..."
export PATH="$TOOLDIR:$PATH"
export DEPLOY_GTK_VERSION=3
export NO_STRIP=1
"$TOOLDIR/linuxdeploy" \
    --appdir "$APPDIR" \
    --executable "$APPDIR/usr/bin/acuity" \
    --desktop-file "$ROOT_DIR/install/linux/acuity.desktop" \
    --icon-file "$ROOT_DIR/ui/static/logo.svg" \
    --plugin gtk

echo "==> Generating AppImage..."
env -u SOURCE_DATE_EPOCH ARCH=x86_64 "$TOOLDIR/linuxdeploy" \
    --appdir "$APPDIR" \
    --output appimage

mv Acuity*.AppImage "$ROOT_DIR/build/bin/Acuity-x86_64.AppImage" 2>/dev/null || \
mv acuity*.AppImage "$ROOT_DIR/build/bin/Acuity-x86_64.AppImage"

echo "==> Success: AppImage created at build/bin/Acuity-x86_64.AppImage"
