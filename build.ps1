# Build dfswitch for macOS (arm64 + amd64) and Windows (amd64) from a Windows host.
# Frontend is rebuilt and embedded via -tags embed.
$ErrorActionPreference = "Stop"

Set-Location $PSScriptRoot

Write-Host "==> Building frontend"
Push-Location web
npm run build
if ($LASTEXITCODE -ne 0) { throw "frontend build failed" }
Pop-Location

New-Item -ItemType Directory -Force -Path dist | Out-Null

$env:CGO_ENABLED = "0"
$flags = @("-tags", "embed", "-trimpath", "-ldflags", "-s -w")

Write-Host "==> Windows amd64"
$env:GOOS = "windows"; $env:GOARCH = "amd64"
go build @flags -o dist/dfswitch.exe .
if ($LASTEXITCODE -ne 0) { throw "windows build failed" }

Write-Host "==> macOS arm64"
$env:GOOS = "darwin"; $env:GOARCH = "arm64"
go build @flags -o dist/dfswitch-darwin-arm64 .
if ($LASTEXITCODE -ne 0) { throw "darwin arm64 build failed" }

Write-Host "==> macOS amd64"
$env:GOOS = "darwin"; $env:GOARCH = "amd64"
go build @flags -o dist/dfswitch-darwin-amd64 .
if ($LASTEXITCODE -ne 0) { throw "darwin amd64 build failed" }

Write-Host ""
Write-Host "Done. Artifacts:"
Get-ChildItem dist
