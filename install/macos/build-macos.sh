#!/usr/bin/env bash
set -e

# Build universal macOS .app bundle and DMG installer
ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT_DIR"

echo "==> Building Acuity Universal Binary for macOS (Apple Silicon + Intel)..."
wails build -platform darwin/universal -clean

echo "==> Application bundle created at build/bin/Acuity.app"

# Optional: Generate DMG if create-dmg is installed (brew install create-dmg)
if command -v create-dmg &>/dev/null; then
    echo "==> Creating .dmg installer disk image..."
    DMG_PATH="$ROOT_DIR/build/bin/Acuity-Universal.dmg"
    rm -f "$DMG_PATH"

    create-dmg \
        --volname "Acuity Installer" \
        --window-pos 200 120 \
        --window-size 600 400 \
        --icon-size 100 \
        --icon "Acuity.app" 175 190 \
        --app-drop-link 425 190 \
        "$DMG_PATH" \
        "$ROOT_DIR/build/bin/Acuity.app"

    echo "==> Success: DMG created at $DMG_PATH"
else
    echo "==> Tip: Install 'create-dmg' (brew install create-dmg) to build .dmg installer image."
fi
