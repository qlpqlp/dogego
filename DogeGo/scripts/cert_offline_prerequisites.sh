#!/usr/bin/env bash
# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# ROADMAP certification exit checklist - offline prerequisites bundle (no dogego-live).
#
# Cross-platform:
#   go run ./cmd/dogego cert offline && go run ./cmd/dogego cert wallet-import
#
#   ./scripts/cert_offline_prerequisites.sh
#   INCLUDE_PQ=1 ./scripts/cert_offline_prerequisites.sh
#   INCLUDE_OPERATOR=1 ./scripts/cert_offline_prerequisites.sh
set -euo pipefail
cd "$(dirname "$0")/.."

echo "=== DogeGo offline prerequisites (ROADMAP exit checklist) ==="

echo
echo "> go run ./cmd/dogego cert offline"
go run ./cmd/dogego cert offline

echo
echo "> go run ./cmd/dogego cert wallet-import"
go run ./cmd/dogego cert wallet-import

if [ "${INCLUDE_PQ:-0}" = "1" ]; then
  echo
  echo "> go run ./cmd/dogego cert pq (optional PQ slice)"
  go run ./cmd/dogego cert pq
fi

if [ "${INCLUDE_OPERATOR:-0}" = "1" ]; then
  echo
  echo "> go run ./cmd/dogego cert operator (deep Milestone E)"
  go run ./cmd/dogego cert operator
fi

echo
echo "Offline prerequisites passed."
