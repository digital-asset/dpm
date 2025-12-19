@echo off
setlocal

set "command=%~1"

if "%command%"=="get" (
    echo {"Username":"meep","Secret":"meep!"}
    exit /b 0
)

:: For store and erase operations
exit /b 1
