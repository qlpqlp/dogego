#!/usr/bin/env bash
# Build dogego.radiodoge universal zip (all platform binaries + icon + docs).
set -euo pipefail
cd "$(dirname "$0")"

dist="dist"
bin_root="$dist/bin"
rm -rf "$bin_root"
mkdir -p "$dist"
export CGO_ENABLED=0

build_one() {
  local key=$1 goos=$2 goarch=$3 out=$4
  local dir="$bin_root/$key"
  mkdir -p "$dir"
  GOOS=$goos GOARCH=$goarch go build -ldflags="-s -w" -trimpath -o "$dir/$out" ./cmd/radiodoge-ext
}

build_one windows-amd64 windows amd64 radiodoge-ext.exe
build_one windows-arm64 windows arm64 radiodoge-ext.exe
build_one linux-amd64   linux   amd64 radiodoge-ext
build_one linux-arm64   linux   arm64 radiodoge-ext
build_one darwin-amd64  darwin  amd64 radiodoge-ext
build_one darwin-arm64  darwin  arm64 radiodoge-ext

manifest="$dist/dogego.extension.json"
python3 - <<'PY'
import json, pathlib
base = json.loads(pathlib.Path("dogego.extension.json").read_text())
base["entry"]["binaries"] = {
    "windows-amd64": "bin/windows-amd64/radiodoge-ext.exe",
    "windows-arm64": "bin/windows-arm64/radiodoge-ext.exe",
    "linux-amd64": "bin/linux-amd64/radiodoge-ext",
    "linux-arm64": "bin/linux-arm64/radiodoge-ext",
    "darwin-amd64": "bin/darwin-amd64/radiodoge-ext",
    "darwin-arm64": "bin/darwin-arm64/radiodoge-ext",
}
pathlib.Path("dist/dogego.extension.json").write_text(json.dumps(base, separators=(",", ":")))
PY

stage="$dist/stage"
rm -rf "$stage"
mkdir -p "$stage"
cp -f "$manifest" "$stage/dogego.extension.json"
cp -f icon.png "$stage/icon.png"
cp -rf "$bin_root" "$stage/bin"
cp -rf docs "$stage/docs"

zip="$dist/radiodoge-universal.zip"
rm -f "$zip"
(cd "$stage" && zip -r -q ../radiodoge-universal.zip dogego.extension.json icon.png bin docs)
rm -rf "$stage"
rm -f "$manifest"
cp "$zip" "$dist/radiodoge.zip"
echo "Wrote $zip ($(wc -c < "$zip") bytes)"
if command -v sha256sum >/dev/null; then
  sha256sum "$zip"
elif command -v shasum >/dev/null; then
  shasum -a 256 "$zip"
fi
