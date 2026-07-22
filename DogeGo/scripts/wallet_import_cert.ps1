# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Offline wallet import certification (BIP39/BIP38 + RPC + UI API).
#
# Cross-platform: dogego cert wallet-import
#   go run ./cmd/dogego cert wallet-import
#
#   .\scripts\wallet_import_cert.ps1
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

Write-Host "=== Wallet import certification (offline) ===" -ForegroundColor Cyan
go run ./cmd/dogego cert wallet-import
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
exit 0
