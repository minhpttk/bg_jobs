# PowerShell script to run Task Recovery Test on Windows
# Usage: .\scripts\test_task_recovery.ps1

Write-Host "🚀 Starting Task Recovery Test on Windows..." -ForegroundColor Green

# Check if Go is installed
try {
    $goVersion = go version
    Write-Host "✅ Go found: $goVersion" -ForegroundColor Green
} catch {
    Write-Host "❌ Go is not installed or not in PATH" -ForegroundColor Red
    exit 1
}

# Check if .env file exists
if (-not (Test-Path ".env")) {
    Write-Host "❌ .env file not found. Please create one with your database configuration." -ForegroundColor Red
    exit 1
}

# Build the test script
Write-Host "📦 Building test script..." -ForegroundColor Yellow
try {
    go build -o bin/test_task_recovery.exe ./scripts/test_task_recovery.go
    Write-Host "✅ Test script built successfully" -ForegroundColor Green
} catch {
    Write-Host "❌ Failed to build test script" -ForegroundColor Red
    exit 1
}

# Run the test
Write-Host "🧪 Running Task Recovery Test..." -ForegroundColor Yellow
try {
    & .\bin\test_task_recovery.exe
    Write-Host "✅ Test completed successfully!" -ForegroundColor Green
} catch {
    Write-Host "❌ Test failed with error: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Clean up
Write-Host "🧹 Cleaning up..." -ForegroundColor Yellow
if (Test-Path "bin\test_task_recovery.exe") {
    Remove-Item "bin\test_task_recovery.exe"
    Write-Host "✅ Cleanup completed" -ForegroundColor Green
}

Write-Host "🎉 Task Recovery Test completed!" -ForegroundColor Green