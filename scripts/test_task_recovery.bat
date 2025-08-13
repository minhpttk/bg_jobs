@echo off
REM Batch script to run Task Recovery Test on Windows
REM Usage: scripts\test_task_recovery.bat

echo 🚀 Starting Task Recovery Test on Windows...

REM Check if Go is installed
go version >nul 2>&1
if %errorlevel% neq 0 (
    echo ❌ Go is not installed or not in PATH
    exit /b 1
)
echo ✅ Go found

REM Check if .env file exists
if not exist ".env" (
    echo ❌ .env file not found. Please create one with your database configuration.
    exit /b 1
)

REM Create bin directory if it doesn't exist
if not exist "bin" mkdir bin

REM Build the test script
echo 📦 Building test script...
go build -o bin\test_task_recovery.exe scripts\test_task_recovery.go
if %errorlevel% neq 0 (
    echo ❌ Failed to build test script
    exit /b 1
)
echo ✅ Test script built successfully

REM Run the test
echo 🧪 Running Task Recovery Test...
bin\test_task_recovery.exe
if %errorlevel% neq 0 (
    echo ❌ Test failed
    exit /b 1
)
echo ✅ Test completed successfully!

REM Clean up
echo 🧹 Cleaning up...
if exist "bin\test_task_recovery.exe" del bin\test_task_recovery.exe
echo ✅ Cleanup completed

echo 🎉 Task Recovery Test completed!