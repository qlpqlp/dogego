#!/usr/bin/env bash
# Build example.wasm release zip.
set -euo pipefail
cd "$(dirname "$0")"

test -f ping.wasm || { echo "ping.wasm missing (see ping.wat)" >&2; exit 1; }
rm -f ping.zip
zip -r ping.zip dogego.extension.json icon.png ping.wasm
echo "Wrote ping.zip ($(wc -c < ping.zip) bytes)"
sha256sum ping.zip | awk '{print "sha256="$1}'
echo "Update extensions/catalog/catalog.json example.wasm sha256 if publishing."
