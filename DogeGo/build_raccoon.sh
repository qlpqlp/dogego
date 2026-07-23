#!/usr/bin/env bash
# Copyright (c) 2026 Paulo Vidal / Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Build DogeGo with Raccoon-G-44 in-tree port (libgmp + libmpfr required).
set -euo pipefail
cd "$(dirname "$0")"
export CGO_ENABLED=1
go build -tags raccoon_g -trimpath -buildvcs=true -o dogego ./cmd/dogego
echo "OK: $(pwd)/dogego (raccoon_g CGO)"
./dogego version
