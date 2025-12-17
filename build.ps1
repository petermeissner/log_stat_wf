#!/usr/bin/env pwsh
# Build script for log_stat_wf - Cross-platform Go build

# Try to get version from git tag, fallback to "dev"
$VERSION = $(git describe)


$APP_NAME = "log_stat_wf"
$DIST_DIR = "distribution"
$SRC_DIR = "src"

# Build targets (OS/ARCH combinations)
$TARGETS = @(
    @{OS="windows"; ARCH="amd64"; EXT=".exe"},
    @{OS="linux"; ARCH="amd64"; EXT=""},
    @{OS="darwin"; ARCH="amd64"; EXT=""},
    @{OS="darwin"; ARCH="arm64"; EXT=""}
)

Write-Host "======================================" -ForegroundColor Cyan
Write-Host " Building $APP_NAME v$VERSION" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""

# Create distribution directory
New-Item -ItemType Directory -Path $DIST_DIR -Force | Out-Null

# Get build timestamp
$BUILD_TIME = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
$GIT_COMMIT = ""
try {
    $GIT_COMMIT = (git rev-parse --short HEAD 2>$null)
    if ($LASTEXITCODE -ne 0) { $GIT_COMMIT = "unknown" }
} catch {
    $GIT_COMMIT = "unknown"
}

Write-Host "Version:     $VERSION" -ForegroundColor Green
Write-Host "Git Commit:  $GIT_COMMIT" -ForegroundColor Green
Write-Host "Build Time:  $BUILD_TIME" -ForegroundColor Green
Write-Host ""

# Build for each target
$successCount = 0
$failCount = 0

foreach ($target in $TARGETS) {
    $os = $target.OS
    $arch = $target.ARCH
    $ext = $target.EXT
    
    $outputName = "${APP_NAME}_${os}_${arch}_${VERSION}${ext}"
    $outputPath = Join-Path $DIST_DIR $outputName
    
    Write-Host "Building for $os/$arch..." -NoNewline
    
    $env:GOOS = $os
    $env:GOARCH = $arch
    $env:CGO_ENABLED = "0"
    
    # Build with ldflags to inject version info
    $ldflags = "-s -w -X 'main.Version=$VERSION' -X 'main.BuildTime=$BUILD_TIME' -X 'main.GitCommit=$GIT_COMMIT'"
    
    try {
        $buildCmd = "go build -ldflags `"$ldflags`" -o `"$outputPath`" ./$SRC_DIR"
        Invoke-Expression $buildCmd 2>&1 | Out-Null
        
        if ($LASTEXITCODE -eq 0 -and (Test-Path $outputPath)) {
            $fileSize = (Get-Item $outputPath).Length
            $fileSizeMB = [math]::Round($fileSize / 1MB, 2)
            Write-Host " OK ($fileSizeMB MB)" -ForegroundColor Green
            $successCount++
        } else {
            Write-Host " FAILED" -ForegroundColor Red
            $failCount++
        }
    } catch {
        Write-Host " FAILED: $_" -ForegroundColor Red
        $failCount++
    }
}

# Clean up environment variables
Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "======================================" -ForegroundColor Cyan
Write-Host " Build Summary" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host "Successful: $successCount" -ForegroundColor Green
Write-Host "Failed:     $failCount" -ForegroundColor $(if ($failCount -gt 0) { "Red" } else { "Green" })
Write-Host ""
Write-Host "Output directory: $DIST_DIR" -ForegroundColor Cyan

# List built files
if ($successCount -gt 0) {
    Write-Host ""
    Write-Host "Built files:" -ForegroundColor Cyan
    Get-ChildItem -Path $DIST_DIR | ForEach-Object {
        $size = [math]::Round($_.Length / 1MB, 2)
        Write-Host "  $($_.Name) - $size MB"
    }
}

Write-Host ""
if ($failCount -eq 0) {
    Write-Host "Build completed successfully!" -ForegroundColor Green
    exit 0
} else {
    Write-Host "Build completed with errors." -ForegroundColor Yellow
    exit 1
}
