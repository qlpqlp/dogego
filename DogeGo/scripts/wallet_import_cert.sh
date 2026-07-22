#!/usr/bin/env bash
# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Offline wallet import certification (BIP39/BIP38 + RPC + UI API).
#
# Cross-platform: dogego cert wallet-import
#   go run ./cmd/dogego cert wallet-import
#
#   ./scripts/wallet_import_cert.sh
set -euo pipefail
cd "$(dirname "$0")/.."

echo "=== Wallet import certification (offline) ==="
go run ./cmd/dogego cert wallet-import

echo "Wallet import certification passed."
