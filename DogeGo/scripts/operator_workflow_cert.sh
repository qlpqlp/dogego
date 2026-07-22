#!/usr/bin/env bash
# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# DogeGo standalone operator certification (offline, no P2P).
#
# Cross-platform: dogego cert operator
#   go run ./cmd/dogego cert operator
#
#   ./scripts/operator_workflow_cert.sh
# Optional live mainnet field disk connect (Windows only): DOGEGO_FIELD_DISK_CONNECT=1 operator_workflow_cert.ps1
set -euo pipefail
cd "$(dirname "$0")/.."

echo "=== DogeGo operator workflow certification ==="
go run ./cmd/dogego cert operator

if [ "${DOGEGO_FIELD_DISK_CONNECT:-}" = "1" ]; then
  echo "ERROR: DOGEGO_FIELD_DISK_CONNECT=1 requires scripts/field_disk_connect_cert.ps1 (Windows PowerShell)." >&2
  echo "Run scripts/operator_workflow_cert.ps1 on Windows for live disk connect cert." >&2
  exit 1
fi

echo "Operator workflow certification passed."
