#!/usr/bin/env bash
# Renders the tap files for a release.
# Usage: render.sh <owner/repo> <version> <source.tar.gz url> <source sha256> <app zip sha256> <out dir>
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
repo="$1" ver="$2" src="$3" srcsha="$4" appsha="$5" out="$6"
mkdir -p "$out/Formula" "$out/Casks"
sub() { sed -e "s#__REPO__#$repo#g" -e "s#__VERSION__#$ver#g" -e "s#__SRC_URL__#$src#g" \
            -e "s#__SRC_SHA__#$srcsha#g" -e "s#__APP_SHA__#$appsha#g" "$1"; }
sub "$here/dikecdpi.rb.tmpl" > "$out/Formula/dikecdpi.rb"
sub "$here/dikec-dpi.rb.tmpl" > "$out/Casks/dikec-dpi.rb"
echo "wrote $out/Formula/dikecdpi.rb and $out/Casks/dikec-dpi.rb"
