#!/usr/bin/env bash
# Build example.go universal zip (all platform binaries + icon).
set -euo pipefail
cd "$(dirname "$0")"

mkdir -p dist/bin
rm -rf dist/bin/*
platforms=(
  "windows-amd64:windows:amd64:hello-ext.exe"
  "windows-arm64:windows:arm64:hello-ext.exe"
  "linux-amd64:linux:amd64:hello-ext"
  "linux-arm64:linux:arm64:hello-ext"
  "darwin-amd64:darwin:amd64:hello-ext"
  "darwin-arm64:darwin:arm64:hello-ext"
)

manifest="$PWD/dist/dogego.extension.json"
python3 - <<'PY' > "$manifest"
import json
with open("dogego.extension.json") as f:
    m = json.load(f)
m["entry"]["binaries"] = {
    "windows-amd64": "bin/windows-amd64/hello-ext.exe",
    "windows-arm64": "bin/windows-arm64/hello-ext.exe",
    "linux-amd64": "bin/linux-amd64/hello-ext",
    "linux-arm64": "bin/linux-arm64/hello-ext",
    "darwin-amd64": "bin/darwin-amd64/hello-ext",
    "darwin-arm64": "bin/darwin-arm64/hello-ext",
}
print(json.dumps(m, indent=2))
PY

for spec in "${platforms[@]}"; do
  IFS=: read -r key goos goarch out <<< "$spec"
  dir="dist/bin/$key"
  mkdir -p "$dir"
  GOOS="$goos" GOARCH="$goarch" go build -ldflags="-s -w" -trimpath -o "$dir/$out" ./hello
done

rm -f dist/hello-universal.zip
mkdir -p dist/stage
cp -f dogego.extension.json icon.png dist/stage/
# rebuild staged manifest with binaries (overwrite plain copy)
cp -f "$manifest" dist/stage/dogego.extension.json
cp -rf dist/bin dist/stage/bin
(cd dist/stage && zip -r ../hello-universal.zip dogego.extension.json icon.png bin/)
rm -rf dist/stage
rm -f "$manifest"
echo "Wrote dist/hello-universal.zip ($(wc -c < dist/hello-universal.zip) bytes)"
sha256sum dist/hello-universal.zip | awk '{print "sha256="$1}'
