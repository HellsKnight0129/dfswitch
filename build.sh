#!/usr/bin/env bash
# Build dfswitch for macOS (arm64 + amd64) and Windows (amd64).
# Frontend is rebuilt and embedded into every binary via -tags embed.
set -euo pipefail

cd "$(dirname "$0")"

echo "==> Building frontend"
(cd web && npm run build)

mkdir -p dist

COMMON_FLAGS=(-tags embed -trimpath -ldflags="-s -w")

echo "==> macOS arm64"
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build "${COMMON_FLAGS[@]}" -o dist/dfswitch-darwin-arm64 .

echo "==> macOS amd64"
GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build "${COMMON_FLAGS[@]}" -o dist/dfswitch-darwin-amd64 .

echo "==> Windows amd64"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build "${COMMON_FLAGS[@]}" -o dist/dfswitch.exe .

echo
echo "Done. Artifacts:"
ls -lh dist/
