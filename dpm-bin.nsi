; installer for bare dpm binary

!include LogicLib.nsh
!define APP_NAME "Dpm"
Outfile "${OUT}"
RequestExecutionLevel admin ; Required for adding to PATH

Section "Install"
    StrCpy $INSTDIR "$APPDATA\dpm\bin"
    CreateDirectory "$INSTDIR"
    SetOutPath "$INSTDIR"
    
    File /oname=dpm.exe "${DPM_BIN_PATH}"

    nsExec::Exec 'echo %PATH% | find "$INSTDIR"'

    ; Use powershell to add %APPDATA%\dpm\bin to the user's PATH if it isn't already in the PATH.
    ; Accounts for when the user's PATH doesn't exist.
    nsExec::ExecToLog `powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "if ([Environment]::GetEnvironmentVariable('path','User') -eq $$null -Or !([Environment]::GetEnvironmentVariable('path','User').split(\";\") -Contains \"$$env:APPDATA\dpm\bin\")) { echo \"Updating PATH\"; [Environment]::SetEnvironmentVariable('path',\"$$env:APPDATA\dpm\bin;$$([Environment]::GetEnvironmentVariable('path','User'))\",'User'); }"`

SectionEnd
