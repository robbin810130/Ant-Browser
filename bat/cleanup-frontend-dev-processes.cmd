@echo off
setlocal EnableExtensions

set "CLEANUP_MODE=%~1"
if not defined CLEANUP_MODE set "CLEANUP_MODE=strict"

echo Cleaning stale frontend dev processes...
node frontend\scripts\dev-port-helper.mjs cleanup
if errorlevel 1 (
    if /I "%CLEANUP_MODE%"=="warn" (
        echo [WARN] Failed to clean stale frontend dev processes. Continuing...
        echo.
        exit /b 0
    )
    echo [ERROR] Failed to clean stale frontend dev processes.
    echo.
    exit /b 1
)
echo.
exit /b 0
