@echo off
setlocal
echo ============================================================
echo   Building Acuity for Windows x64 (Wails + NSIS Installer)
echo ============================================================

cd /d "%~dp0\..\.."

REM Check if wails is installed
where wails >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo Error: Wails CLI is not found in PATH.
    echo Install it with: go install github.com/wailsapp/wails/v2/cmd/wails@latest
    exit /b 1
)

echo Building Windows executable and NSIS setup installer...
wails build -platform windows/amd64 -nsis -clean

if %ERRORLEVEL% equ 0 (
    echo.
    echo ========================================================
    echo   Build succeeded! Installer is in: build\bin\
    echo ========================================================
) else (
    echo.
    echo [!] Build failed with error code %ERRORLEVEL%.
)

pause
