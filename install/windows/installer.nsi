; NSIS Installer Script for Acuity
!include "MUI2.nsh"
!include "FileFunc.nsh"

Name "Acuity"
OutFile "..\..\build\bin\Acuity-Setup.exe"
InstallDir "$PROGRAMFILES64\Acuity"
InstallDirRegKey HKLM "Software\Acuity" "Install_Dir"
RequestExecutionLevel admin

!define MUI_ABORTWARNING
!define MUI_ICON "..\..\build\appicon.ico"
!define MUI_UNICON "..\..\build\appicon.ico"

; Pages
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "German"

Section "Acuity (required)"
    SectionIn RO
    SetOutPath "$INSTDIR"
    File "..\..\build\bin\acuity.exe"
    
    ; Create Start Menu & Desktop Shortcuts
    CreateDirectory "$SMPROGRAMS\Acuity"
    CreateShortcut "$SMPROGRAMS\Acuity\Acuity.lnk" "$INSTDIR\acuity.exe"
    CreateShortcut "$SMPROGRAMS\Acuity\Uninstall.lnk" "$INSTDIR\Uninstall.exe"
    CreateShortcut "$DESKTOP\Acuity.lnk" "$INSTDIR\acuity.exe"
    
    ; Uninstaller registry keys
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Acuity" "DisplayName" "Acuity Photo Organizer"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Acuity" "UninstallString" '"$INSTDIR\Uninstall.exe"'
    WriteUninstaller "$INSTDIR\Uninstall.exe"
SectionEnd

Section "Uninstall"
    Delete "$INSTDIR\acuity.exe"
    Delete "$INSTDIR\Uninstall.exe"
    Delete "$SMPROGRAMS\Acuity\Acuity.lnk"
    Delete "$SMPROGRAMS\Acuity\Uninstall.lnk"
    Delete "$DESKTOP\Acuity.lnk"
    RMDir "$SMPROGRAMS\Acuity"
    RMDir "$INSTDIR"
    DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Acuity"
SectionEnd
