#!/usr/bin/env bash
# Build dogego.radiodoge zip for this OS only.
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p dist
export CGO_ENABLED=0
out="dist/radiodoge-ext"
case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*) out="dist/radiodoge-ext.exe" ;;
esac
go build -ldflags="-s -w" -trimpath -o "$out" ./cmd/radiodoge-ext
tmp="$(mktemp -d)"
cp dogego.extension.json icon.png "$tmp/"
cp -r docs "$tmp/docs"
cp "$out" "$tmp/"
rm -f dist/radiodoge.zip
(cd "$tmp" && zip -qr "$OLDPWD/dist/radiodoge.zip" .)
rm -rf "$tmp"
echo "Wrote dist/radiodoge.zip"
