#!/usr/bin/env bash
# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Core wallet.dat migration certification (native BDB probe/import).
#
# Cross-platform: dogego cert wallet-migration
#   go run ./cmd/dogego cert wallet-migration -offline-only
#   go run ./cmd/dogego cert wallet-migration -skip-offline -live-import   # when DOGEGO_WALLET_DAT set
#
#   ./scripts/wallet_migration_cert.sh
# Env: DOGEGO_WALLET_DAT, DOGEGO_WALLET_DAT_PASSPHRASE, DOGEGO_WALLET_DAT_REQUIRED=1
# Skip offline: SKIP_OFFLINE=1 ./scripts/wallet_migration_cert.sh
set -euo pipefail
cd "$(dirname "$0")/.."

echo "=== Core wallet.dat migration certification ==="

if [ "${SKIP_OFFLINE:-0}" != "1" ]; then
  go run ./cmd/dogego cert wallet-migration -offline-only
fi

wallet_dat="${DOGEGO_WALLET_DAT:-}"
if [ -n "$wallet_dat" ] && [ -f "$wallet_dat" ]; then
  args=(-skip-offline -live-import)
  if [ "${DOGEGO_WALLET_DAT_REQUIRED:-0}" = "1" ] || [ "${REQUIRE_WALLET_DAT:-0}" = "1" ]; then
    args+=(-require-wallet-dat)
  fi
  if [ -n "${DOGEGO_WALLET_DAT_PASSPHRASE:-}" ]; then
    args+=(-passphrase "$DOGEGO_WALLET_DAT_PASSPHRASE")
  fi
  go run ./cmd/dogego cert wallet-migration "${args[@]}"
elif [ -n "$wallet_dat" ]; then
  echo "Wallet path not found (skipping live probe): $wallet_dat" >&2
elif [ "${DOGEGO_WALLET_DAT_REQUIRED:-0}" = "1" ] || [ "${REQUIRE_WALLET_DAT:-0}" = "1" ]; then
  echo "wallet.dat required but not found (set DOGEGO_WALLET_DAT)" >&2
  exit 1
fi

echo "Wallet migration certification passed."
