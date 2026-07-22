#!/usr/bin/env bash
# Build example.go release zip (subprocess extension).
# Output goes to dist/ so the package root stays source-only.
set -euo pipefail
cd "$(dirname "$0")"

mkdir -p dist
bin=dist/hello-ext
go build -ldflags="-s -w" -trimpath -o "$bin" ./hello
rm -f dist/hello.zip
(cd dist && zip -r hello.zip ../dogego.extension.json ../icon.png hello-ext)
echo "Wrote dist/hello.zip ($(wc -c < dist/hello.zip) bytes)"
sha256sum dist/hello.zip | awk '{print "sha256="$1}'
echo "Install: Settings -> Extensions -> Install zip -> select dist/hello.zip"
