#!/usr/bin/env bash
set -e

# Build universal .app bundle & .dmg installer for macOS (Apple Silicon + Intel)
ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT_DIR"

echo "==> Building universal Acuity.app with Wails..."
wails build -clean -platform darwin/universal

APP="$ROOT_DIR/build/bin/Acuity.app"
DMG_STAGING="$ROOT_DIR/build/dmg-staging"
DMG_OUT="$ROOT_DIR/build/bin/Acuity-Universal.dmg"

rm -rf "$DMG_STAGING"
mkdir -p "$DMG_STAGING"
cp -R "$APP" "$DMG_STAGING/"
ln -s /Applications "$DMG_STAGING/Applications"
rm -f "$DMG_OUT"

if command -v create-dmg &>/dev/null; then
    echo "==> Generating DMG with create-dmg..."
    create-dmg \
        --volname "Acuity" \
        --window-size 600 400 \
        --icon-size 100 \
        --icon "Acuity.app" 150 200 \
        --app-drop-link 450 200 \
        --hide-extension "Acuity.app" \
        "$DMG_OUT" \
        "$DMG_STAGING" || true
else
    echo "==> create-dmg not found, falling back to hdiutil..."
    hdiutil create -volname "Acuity" -srcfolder "$DMG_STAGING" -ov -format UDZO "$DMG_OUT"
fi

echo "==> Success: DMG created at build/bin/Acuity-Universal.dmg"
echo "==> Note: the app is unsigned. Recipients must right-click -> Open on"
echo "    first launch (or run 'xattr -cr /Applications/Acuity.app') to bypass"
echo "    Gatekeeper's 'unidentified developer' warning."
