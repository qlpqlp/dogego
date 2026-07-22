#!/usr/bin/env bash
# Build dogego.zkl2 source zip (manifest + icon + Go sources + docs).
set -euo pipefail
cd "$(dirname "$0")"

mkdir -p dist
rm -f dist/zkl2.zip
zip -r dist/zkl2.zip dogego.extension.json icon.png docs/README.md docs/USER_GUIDE.md docs/PROTOCOL.md ./*.go
echo "Wrote dist/zkl2.zip ($(wc -c < dist/zkl2.zip) bytes)"
sha256sum dist/zkl2.zip | awk '{print "sha256="$1}'
