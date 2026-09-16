# Acuity Packaging & Distribution

This directory contains build scripts and package manifests for building
Acuity installers on Linux, Windows, and macOS.

---

## For most people: download a release

The easiest way to get Acuity is to download a pre-built installer from the
[Releases page](https://github.com/schlks/acuity/releases) on GitHub. Every
tagged release (`vX.Y.Z`) is built automatically for all three platforms by
`.github/workflows/release.yml` and attached to a draft release for review
before publishing.

- **Linux:** `Acuity-x86_64.AppImage` (chmod +x, double-click) or
  `acuity_<version>_amd64.deb` (`apt install ./acuity_*.deb`)
- **Windows:** `acuity-amd64-installer.exe`
- **macOS:** `Acuity-Universal.dmg` — unsigned, so on first launch you need to
  right-click the app → Open (or run `xattr -cr /Applications/Acuity.app`) to
  get past Gatekeeper's "unidentified developer" warning.

The sections below are for building these yourself.

---

## Linux

### 1. Arch Linux (PKGBUILD)

```bash
cd install/linux
makepkg -si
```

### 2. Standalone AppImage (Universal Linux)

```bash
chmod +x install/linux/build-appimage.sh
./install/linux/build-appimage.sh
```

Output: `build/bin/Acuity-x86_64.AppImage`

Runtime dependencies (libvips, WebKitGTK, GTK modules) are bundled into the
AppImage via `linuxdeploy`, so it should run on any modern x86_64 Linux
distribution without extra installs.

### 3. Debian / Ubuntu (.deb)

```bash
chmod +x install/linux/build-deb.sh
./install/linux/build-deb.sh
```

Output: `build/bin/acuity_0.1.0_amd64.deb`. `apt install ./build/bin/acuity_*.deb`
resolves the declared dependencies (`libwebkit2gtk-4.1-0`, `libgtk-3-0`,
`libvips42`, `libsqlite3-0`) automatically.

---

## Windows

Build directly with the Wails CLI (requires Go, Bun, and a libvips-for-Windows
build on `PKG_CONFIG_PATH`/`CGO_CFLAGS`/`CGO_LDFLAGS` — see the `windows` job
in `.github/workflows/release.yml` for the exact setup):

```powershell
wails build -platform windows/amd64 -nsis
```

Output: `build\bin\acuity-amd64-installer.exe` and `build\bin\acuity.exe`.
Wails scaffolds a default NSIS installer script into
`build/windows/installer/project.nsi` on first run if it doesn't exist yet
(NSIS is required — `choco install nsis` if you don't have it).

---

## macOS

```bash
chmod +x install/macos/build-macos.sh
./install/macos/build-macos.sh
```

Output: `build/bin/Acuity.app` and `build/bin/Acuity-Universal.dmg` (Apple
Silicon + Intel). Requires libvips (`brew install vips`) and, optionally,
`create-dmg` (`brew install create-dmg`) for a nicer installer window — the
script falls back to plain `hdiutil` if `create-dmg` isn't installed.

The app is unsigned (no Apple Developer Program membership configured yet),
so Gatekeeper will warn on first launch — see the note above.

---

### AI was used to assist writing code
