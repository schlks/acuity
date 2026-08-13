# Acuity Packaging & Distribution

This directory contains build scripts, package manifests, and installers for Linux, Windows, and macOS.

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

### 3. Debian / Ubuntu (.deb)

```bash
chmod +x install/linux/build-deb.sh
./install/linux/build-deb.sh
```

Output: `build/bin/acuity_0.1.0_amd64.deb`

---

## Windows

### Build Windows Binary & Setup Installer (.exe)

Double-click or run:

```cmd
install\windows\build-windows.bat
```

Or with PowerShell:

```powershell
.\install\windows\build-windows.ps1
```

Output: `build\bin\acuity-amd64-installer.exe` and `build\bin\acuity.exe`

---

## macOS

### Build Universal .app Bundle & .dmg Installer (Apple Silicon + Intel)

```bash
chmod +x install/macos/build-macos.sh
./install/macos/build-macos.sh
```

Output: `build/bin/Acuity.app` and `build/bin/Acuity-Universal.dmg`
