#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p dist
go build -ldflags="-s -w" -trimpath -o dist/doginals-ext ./cmd/doginals-ext
rm -f dist/doginals.zip
(
  cd dist
  cp -f ../dogego.extension.json ../icon.png .
  cp -rf ../docs .
  zip -r doginals.zip dogego.extension.json icon.png docs doginals-ext
  rm -f dogego.extension.json icon.png
  rm -rf docs
)
echo "Wrote dist/doginals.zip"
sha256sum dist/doginals.zip 2>/dev/null || shasum -a 256 dist/doginals.zip
