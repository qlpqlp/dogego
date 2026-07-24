#!/usr/bin/env bash
# Copyright (c) 2026 Paulo Vidal / Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Build DogeGo with Raccoon-G-44 in-tree port (libgmp + libmpfr required).
set -euo pipefail
cd "$(dirname "$0")"
export CGO_ENABLED=1

# macOS: #cgo does not pass -lgmp/-lmpfr (avoids accidental .dylib pull in release CI).
# Local/dev builds need Homebrew search paths explicitly.
if [[ "$(uname -s)" == "Darwin" ]]; then
  if command -v brew >/dev/null 2>&1; then
    GMP="$(brew --prefix gmp)"
    MPFR="$(brew --prefix mpfr)"
    ZSTD="$(brew --prefix zstd 2>/dev/null || true)"
    export CGO_CFLAGS="-I${GMP}/include -I${MPFR}/include${ZSTD:+ -I${ZSTD}/include}"
    export CGO_LDFLAGS="-L${GMP}/lib -L${MPFR}/lib${ZSTD:+ -L${ZSTD}/lib} -lgmp -lmpfr${ZSTD:+ -lzstd}"
  else
    echo "macOS: install Homebrew gmp mpfr (and zstd) or set CGO_CFLAGS/CGO_LDFLAGS" >&2
    exit 1
  fi
fi

go build -tags raccoon_g -trimpath -buildvcs=true -o dogego ./cmd/dogego
echo "OK: $(pwd)/dogego (raccoon_g CGO)"
./dogego version
