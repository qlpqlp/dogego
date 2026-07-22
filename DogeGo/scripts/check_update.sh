#!/usr/bin/env bash
# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Query GitHub for a newer DogeGo release (exit 2 when update available).
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ "${1:-}" == "-json" ]]; then
  exec go run ./cmd/dogego version -json
fi
exec go run ./cmd/dogego version -check
