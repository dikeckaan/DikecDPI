#!/usr/bin/env bash
# Builds Dikec DPI for Windows (no cgo needed) into gui/dist-windows:
#   DikecDPI.exe   desktop app (menu bar = system tray)
#   cli/dikecdpi.exe  command-line tool (separate folder: Windows and macOS file names are case-insensitive)
# Usage: gui/build/package-windows.sh [version] [arch: amd64|arm64]
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${1:-$(cat ../VERSION 2>/dev/null || echo dev)}"
ARCH="${2:-amd64}"
OUT="dist-windows"
rm -rf "$OUT" && mkdir -p "$OUT/DikecDPI/cli"

# App icon + version info embedded into the .exe (rsrc .syso picked up by go build).
go run . -render-icon "$OUT/icon.png" -icon-size 256
go run github.com/tc-hib/go-winres@v0.3.3 simply \
  --arch "$ARCH" --out rsrc --icon "$OUT/icon.png" \
  --manifest gui --product-name "Dikec DPI" --file-description "Dikec DPI" \
  --product-version "$VERSION" --file-version "$VERSION" --original-filename DikecDPI.exe \
  --copyright "Apache 2.0"

GOOS=windows GOARCH="$ARCH" CGO_ENABLED=0 go build -trimpath -tags production \
  -ldflags "-s -w -H windowsgui -X main.version=$VERSION" -o "$OUT/DikecDPI/DikecDPI.exe" .
rm -f rsrc_windows_*.syso

( cd .. && GOOS=windows GOARCH="$ARCH" CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=$VERSION -X main.build=release" -o "gui/$OUT/DikecDPI/cli/dikecdpi.exe" ./cmd/spoofdpi )

cp ../LICENSE "$OUT/DikecDPI/LICENSE.txt"
( cd "$OUT" && zip -qr "DikecDPI-$VERSION-windows-$ARCH.zip" DikecDPI )
rm -f "$OUT/icon.png"
ls -la "$OUT"
