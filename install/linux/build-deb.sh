#!/usr/bin/env bash
set -e

# Build .deb package for Debian / Ubuntu
ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT_DIR"

VERSION="0.1.0"
ARCH="amd64"
DEB_DIR="$ROOT_DIR/build/deb/acuity_${VERSION}_${ARCH}"

echo "==> Building Acuity binary with Wails..."
wails build -clean -tags webkit2_41

rm -rf "$DEB_DIR"
mkdir -p "$DEB_DIR/DEBIAN"
mkdir -p "$DEB_DIR/usr/bin"
mkdir -p "$DEB_DIR/usr/share/applications"
mkdir -p "$DEB_DIR/usr/share/icons/hicolor/scalable/apps"

# Control file
cat << EOF > "$DEB_DIR/DEBIAN/control"
Package: acuity
Version: ${VERSION}
Section: graphics
Priority: optional
Architecture: ${ARCH}
Depends: libwebkit2gtk-4.1-0, libgtk-3-0, libvips42, libsqlite3-0
Maintainer: shlks
Description: Fast local photo organizer with AI visual & semantic search
 Acuity provides instant semantic and visual image search powered by CLIP and sqlite-vec.
EOF

# Copy binaries and assets
cp "$ROOT_DIR/build/bin/acuity" "$DEB_DIR/usr/bin/acuity"
cp "$ROOT_DIR/install/linux/acuity.desktop" "$DEB_DIR/usr/share/applications/acuity.desktop"
cp "$ROOT_DIR/ui/static/logo.svg" "$DEB_DIR/usr/share/icons/hicolor/scalable/apps/acuity.svg"

# Build debian package
echo "==> Packaging .deb..."
dpkg-deb --build --root-owner-group "$DEB_DIR" "$ROOT_DIR/build/bin/acuity_${VERSION}_${ARCH}.deb"
echo "==> Success: Debian package created at build/bin/acuity_${VERSION}_${ARCH}.deb"
