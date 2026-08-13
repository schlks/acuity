# Build Acuity for Windows x64 via PowerShell
$ErrorActionPreference = "Stop"

$rootDir = Resolve-Path "$PSScriptRoot\..\.."
Set-Location $rootDir

Write-Host "==> Checking dependencies..." -ForegroundColor Cyan
if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
    Write-Error "Wails CLI not found. Install it with: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
}

Write-Host "==> Building Windows binary and NSIS Installer..." -ForegroundColor Cyan
wails build -platform windows/amd64 -nsis -clean

Write-Host "==> Build successful! Installer available in build/bin/" -ForegroundColor Green
