@echo off
REM Qube Desktop Installation Script for Windows

echo ========================================
echo    Qube Desktop Installation Script
echo ========================================
echo.

REM Check if Node.js is installed
where node >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] Node.js is not installed!
    echo.
    echo Please install Node.js 16+ from:
    echo https://nodejs.org/
    echo.
    pause
    exit /b 1
)

echo [OK] Node.js version:
node -v
echo.

REM Navigate to script directory
cd /d "%~dp0"

echo [INFO] Installing dependencies...
call npm install

if %errorlevel% neq 0 (
    echo [ERROR] Failed to install dependencies
    pause
    exit /b 1
)

echo.
echo [INFO] Building Windows application...
call npm run build:win

if %errorlevel% neq 0 (
    echo [ERROR] Failed to build application
    pause
    exit /b 1
)

echo.
echo ========================================
echo   Build completed successfully!
echo ========================================
echo.
echo Installation packages created in .\dist\
echo.
echo To install:
echo   1. Run: dist\Qube-Desktop-Setup-*.exe
echo   OR
echo   2. Use portable: dist\Qube-Desktop-*.exe
echo.
echo Done! Qube Desktop is ready to install.
echo.
pause
