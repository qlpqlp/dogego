#!/usr/bin/env bash
# Build official catalog extension zips and refresh catalog.json hashes / download URLs.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CATALOG="$ROOT/extensions/catalog"
JSON="$CATALOG/catalog.json"
export CGO_ENABLED="${CGO_ENABLED:-0}"

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

update_json_hash() {
  local id="$1" ziprel="$2" field="${3:-sha256}"
  local zip="$CATALOG/$ziprel"
  local hash
  hash="$(sha256_file "$zip")"
  python3 - "$JSON" "$id" "$hash" "$field" "$ziprel" <<'PY'
import json, sys, datetime
path, ext_id, digest, field, ziprel = sys.argv[1:6]
with open(path, encoding="utf-8") as f:
    data = json.load(f)
url = f"https://raw.githubusercontent.com/qlpqlp/dogego/main/DogeGo/extensions/catalog/{ziprel.replace(chr(92), '/')}"
for e in data.get("extensions", []):
    if e.get("id") != ext_id:
        continue
    if field == "sha256":
        e["sha256"] = digest
        e["download_url"] = url
    elif field == "universal":
        e.setdefault("downloads", {})
        e["downloads"]["universal"] = {"download_url": url, "sha256": digest}
    break
data["updated"] = datetime.date.today().isoformat()
with open(path, "w", encoding="utf-8", newline="\n") as f:
    json.dump(data, f, indent=2)
    f.write("\n")
print(f"updated {ext_id} {field}={digest}")
PY
}

echo "==> doginals"
(
  cd "$CATALOG/doginals"
  mkdir -p dist
  go build -ldflags="-s -w" -trimpath -o dist/doginals-ext ./cmd/doginals-ext
  rm -f dist/doginals.zip
  (cd . && zip -qr dist/doginals.zip dogego.extension.json icon.png docs dist/doginals-ext)
  # Prefer root-layout zip for installers: rebuild with binary at root of archive.
  rm -f dist/doginals.zip
  tmp="$(mktemp -d)"
  cp dogego.extension.json icon.png "$tmp/"
  cp -r docs "$tmp/docs"
  cp dist/doginals-ext "$tmp/doginals-ext"
  (cd "$tmp" && zip -qr "$CATALOG/doginals/dist/doginals.zip" .)
  rm -rf "$tmp"
)
update_json_hash "dogego.doginals" "doginals/dist/doginals.zip" sha256

echo "==> zkl2"
if [[ -f "$CATALOG/zkl2/build-universal.sh" ]]; then
  bash "$CATALOG/zkl2/build-universal.sh" || bash "$CATALOG/zkl2/build.sh"
elif [[ -f "$CATALOG/zkl2/build.sh" ]]; then
  bash "$CATALOG/zkl2/build.sh"
fi
if [[ -f "$CATALOG/zkl2/dist/zkl2.zip" ]]; then
  update_json_hash "dogego.zkl2" "zkl2/dist/zkl2.zip" sha256
fi
if [[ -f "$CATALOG/zkl2/dist/zkl2-universal.zip" ]]; then
  update_json_hash "dogego.zkl2" "zkl2/dist/zkl2-universal.zip" universal
fi

echo "==> bbpow"
if [[ -f "$CATALOG/bbpow/build.sh" ]]; then
  bash "$CATALOG/bbpow/build.sh" || true
fi
if [[ -f "$CATALOG/bbpow/dist/bbpow.zip" ]]; then
  # optional entry may lack download_url; still hash if present
  update_json_hash "dogego.bbpow" "bbpow/dist/bbpow.zip" sha256 || true
fi

echo "==> radiodoge"
if [[ -f "$CATALOG/radiodoge/build-universal.sh" ]]; then
  bash "$CATALOG/radiodoge/build-universal.sh" || true
fi
if [[ -f "$CATALOG/radiodoge/dist/radiodoge.zip" ]]; then
  update_json_hash "dogego.radiodoge" "radiodoge/dist/radiodoge.zip" sha256 || true
fi
if [[ -f "$CATALOG/radiodoge/dist/radiodoge-universal.zip" ]]; then
  update_json_hash "dogego.radiodoge" "radiodoge/dist/radiodoge-universal.zip" universal
fi

echo "==> example-go"
if [[ -f "$CATALOG/example-go/build-universal.sh" ]]; then
  bash "$CATALOG/example-go/build-universal.sh" || true
fi
if [[ -f "$CATALOG/example-go/dist/hello-universal.zip" ]]; then
  update_json_hash "example.go" "example-go/dist/hello-universal.zip" universal
fi

echo "==> example-wasm"
if [[ -f "$CATALOG/example-wasm/build.sh" ]]; then
  bash "$CATALOG/example-wasm/build.sh" || true
fi

echo "Catalog build complete: $JSON"
