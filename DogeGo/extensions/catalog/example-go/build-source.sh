#!/usr/bin/env bash
# Build example.go source zip (manifest + icon + hello/).
set -euo pipefail
cd "$(dirname "$0")"

mkdir -p dist
rm -f dist/hello-source.zip
zip -r dist/hello-source.zip dogego.extension.json icon.png hello/
echo "Wrote dist/hello-source.zip ($(wc -c < dist/hello-source.zip) bytes)"
sha256sum dist/hello-source.zip | awk '{print "sha256="$1}'
