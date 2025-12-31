# WikiGraph build script for PowerShell

param(
    [Parameter(Position=0)]
    [string]$Command = "help"
)

switch ($Command) {
    "build" {
        Write-Host "Building WikiGraph..."
        go build -o wikigraph ./cmd/server
        if ($LASTEXITCODE -eq 0) {
            Write-Host "Build successful!" -ForegroundColor Green
        }
    }
    "run" {
        Write-Host "Running WikiGraph..."
        go run ./cmd/server
    }
    "test" {
        Write-Host "Running tests..."
        go test ./...
    }
    "clean" {
        Write-Host "Cleaning build artifacts..."
        if (Test-Path "wikigraph.exe") {
            Remove-Item "wikigraph.exe"
            Write-Host "Removed wikigraph.exe" -ForegroundColor Green
        }
        if (Test-Path "wikigraph") {
            Remove-Item "wikigraph"
            Write-Host "Removed wikigraph" -ForegroundColor Green
        }
    }
    default {
        Write-Host "WikiGraph Build Script" -ForegroundColor Cyan
        Write-Host ""
        Write-Host "Usage: .\build.ps1 [command]"
        Write-Host ""
        Write-Host "Commands:"
        Write-Host "  build  - Build the wikigraph binary"
        Write-Host "  run    - Run the application"
        Write-Host "  test   - Run tests"
        Write-Host "  clean  - Remove build artifacts"
    }
}

