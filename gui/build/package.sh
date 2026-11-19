#!/usr/bin/env bash
# Builds "Dikec DPI.app" (and a zip for releases) into gui/dist.
# Usage: gui/build/package.sh [version] [arch: arm64|amd64|universal]
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${1:-$(cat ../VERSION 2>/dev/null || echo dev)}"
ARCH="${2:-$(uname -m | sed 's/x86_64/amd64/')}"
APP="dist/Dikec DPI.app"
export MACOSX_DEPLOYMENT_TARGET=13.0
export CGO_ENABLED=1
export CGO_CFLAGS="-mmacosx-version-min=13.0"
export CGO_LDFLAGS="-mmacosx-version-min=13.0"

rm -rf dist && mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

build() {
  GOOS=darwin GOARCH="$1" go build -trimpath -tags production \
    -ldflags "-s -w -X main.version=$VERSION" -o "$2" .
}
# The CLI core runs TUN mode as root (see tun.go). Built from the parent module.
build_core() {
  ( cd .. && GOOS=darwin GOARCH="$1" go build -trimpath \
    -ldflags "-s -w -X main.version=$VERSION -X main.build=app" -o "gui/$2" ./cmd/spoofdpi )
}
if [ "$ARCH" = "universal" ]; then
  build arm64 dist/DikecDPI-arm64
  build amd64 dist/DikecDPI-amd64
  build_core arm64 dist/core-arm64
  build_core amd64 dist/core-amd64
  lipo -create -output "$APP/Contents/MacOS/DikecDPI" dist/DikecDPI-arm64 dist/DikecDPI-amd64
  lipo -create -output "$APP/Contents/MacOS/dikecdpi-core" dist/core-arm64 dist/core-amd64
  rm dist/DikecDPI-arm64 dist/DikecDPI-amd64 dist/core-arm64 dist/core-amd64
else
  build "$ARCH" "$APP/Contents/MacOS/DikecDPI"
  build_core "$ARCH" "$APP/Contents/MacOS/dikecdpi-core"
fi

# App icon: rendered by the app itself (the "Kesik" mark), then iconutil.
ICONSET=dist/AppIcon.iconset
mkdir -p "$ICONSET"
go run . -render-icon dist/icon-1024.png -icon-size 1024
for s in 16 32 128 256 512; do
  sips -z $s $s dist/icon-1024.png --out "$ICONSET/icon_${s}x${s}.png" >/dev/null
  d=$((s * 2))
  sips -z $d $d dist/icon-1024.png --out "$ICONSET/icon_${s}x${s}@2x.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/AppIcon.icns"
rm -rf "$ICONSET" dist/icon-1024.png

sed "s/__VERSION__/$VERSION/g" build/Info.plist > "$APP/Contents/Info.plist"

# Ad-hoc signature (required on Apple Silicon). Not notarized.
codesign --force --deep --sign - "$APP"

( cd dist && ditto -c -k --keepParent "Dikec DPI.app" "DikecDPI-$VERSION-macos-$ARCH.zip" )
echo "built: $APP"
ls -la dist
