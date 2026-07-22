#!/usr/bin/env bash
# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone A: offline mainnet field evidence certification (no datadir, no RPC).
#
# Cross-platform: dogego cert field-evidence
#   go run ./cmd/dogego cert field-evidence
#
#   ./scripts/field_evidence_cert.sh
# Regenerate testdata (maintainers): UPDATE_CORE_TESTDATA=1 go test ./consensus -run TestUpdateCoreTestdata -count=1
# Auxpow export + regen flags: use field_evidence_cert.ps1 on Windows.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "=== Mainnet field evidence certification (offline) ==="
go run ./cmd/dogego cert field-evidence

echo "Mainnet field evidence certification passed."
