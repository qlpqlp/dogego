#!/usr/bin/env bash
# Build dogego.zkl2 universal zip (all platform binaries + icon + manifest).
set -euo pipefail
cd "$(dirname "$0")"

dist="dist"
bin_root="$dist/bin"
rm -rf "$bin_root"
mkdir -p "$dist"

build_one() {
  local key=$1 goos=$2 goarch=$3 out=$4
  local dir="$bin_root/$key"
  mkdir -p "$dir"
  GOOS=$goos GOARCH=$goarch CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o "$dir/$out" ./cmd/zkl2-ext
}

build_one windows-amd64 windows amd64 zkl2-ext.exe
build_one windows-arm64 windows arm64 zkl2-ext.exe
build_one linux-amd64   linux   amd64 zkl2-ext
build_one linux-arm64   linux   arm64 zkl2-ext
build_one darwin-amd64  darwin  amd64 zkl2-ext
build_one darwin-arm64  darwin  arm64 zkl2-ext

manifest="$dist/dogego.extension.json"
python3 - <<'PY'
import json, pathlib
base = json.loads(pathlib.Path("dogego.extension.json").read_text())
base["entry"]["binaries"] = {
    "windows-amd64": "bin/windows-amd64/zkl2-ext.exe",
    "windows-arm64": "bin/windows-arm64/zkl2-ext.exe",
    "linux-amd64": "bin/linux-amd64/zkl2-ext",
    "linux-arm64": "bin/linux-arm64/zkl2-ext",
    "darwin-amd64": "bin/darwin-amd64/zkl2-ext",
    "darwin-arm64": "bin/darwin-arm64/zkl2-ext",
}
pathlib.Path("dist/dogego.extension.json").write_text(json.dumps(base, separators=(",", ":")))
PY

zip="$dist/zkl2-universal.zip"
rm -f "$zip"
(cd "$dist" && zip -r -q zkl2-universal.zip dogego.extension.json)
(cd "$(pwd)" && zip -r -q "$zip" icon.png bin)
rm -f "$dist/dogego.extension.json"
cp "$zip" "$dist/zkl2.zip"
echo "Wrote $zip ($(wc -c < "$zip") bytes)"
if command -v sha256sum >/dev/null; then
  sha256sum "$zip"
fi
