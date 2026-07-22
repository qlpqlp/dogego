#!/usr/bin/env bash
# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# DogeGo offline CI gate (Linux/macOS). No live node required.
#
# Cross-platform: dogego cert offline
#   go run ./cmd/dogego cert offline
#
#   ./scripts/ci_offline_gate.sh
set -euo pipefail
cd "$(dirname "$0")/.."

echo "=== DogeGo offline CI gate ==="
go run ./cmd/dogego cert offline

echo "Offline CI gate passed."
