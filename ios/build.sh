#!/bin/sh
# Builds the Go core for iOS and generates the Xcode project.
# Needs Xcode, Go, gomobile (go install golang.org/x/mobile/cmd/gomobile@latest)
# and XcodeGen (brew install xcodegen).
set -eu
cd "$(dirname "$0")/.."
export PATH="$(go env GOPATH)/bin:$PATH"
VERSION=$(cat VERSION)
gomobile init 2>/dev/null || true
gomobile bind -target=ios/arm64,iossimulator/arm64 -iosversion 16.0 \
  -ldflags "-s -w -X github.com/xvzc/spoofdpi/mobile.version=${VERSION}" \
  -o ios/Frameworks/Dikeccore.xcframework ./mobile
sed -i '' "s/MARKETING_VERSION: \".*\"/MARKETING_VERSION: \"${VERSION}\"/" ios/project.yml
python3 ios/tools/gen_strings.py >/dev/null
cd ios && xcodegen
echo "Open ios/DikecDPI.xcodeproj, choose your team, connect your iPhone and press Run."
