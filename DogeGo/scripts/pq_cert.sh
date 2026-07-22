#!/usr/bin/env bash
# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Offline PQ format/carrier certification (no production PQ safety claim).
#
# Cross-platform: dogego cert pq
#   go run ./cmd/dogego cert pq
#
#   ./scripts/pq_cert.sh
set -euo pipefail
cd "$(dirname "$0")/.."

echo "=== PQ format/carrier certification (offline) ==="
go run ./cmd/dogego cert pq

echo "PQ certification passed (format/carrier only; no production PQ safety claim)."
