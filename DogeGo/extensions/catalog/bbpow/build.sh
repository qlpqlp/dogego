#!/usr/bin/env bash
# Build dogego.bbpow release zip (subprocess research extension).
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p dist
BIN=dist/bbpow-ext
go build -ldflags="-s -w" -trimpath -o "$BIN" ./cmd/bbpow-ext
ZIP=dist/bbpow.zip
rm -f "$ZIP"
# portable zip: prefer zip(1)
if command -v zip >/dev/null 2>&1; then
  zip -r "$ZIP" dogego.extension.json icon.png docs "$(basename "$BIN")" -x '*.DS_Store'
  # zip from dist for binary path
  rm -f "$ZIP"
  (
    cd dist
    cp -f ../dogego.extension.json ../icon.png .
    cp -rf ../docs .
    zip -r bbpow.zip dogego.extension.json icon.png docs bbpow-ext
    rm -f dogego.extension.json icon.png
    rm -rf docs
  )
else
  echo "zip required" >&2
  exit 1
fi
echo "Wrote $ZIP ($(wc -c < "$ZIP") bytes)"
sha256sum "$ZIP" 2>/dev/null || shasum -a 256 "$ZIP"
echo "Install on TESTNET only"
