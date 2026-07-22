# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Offline PQ format/carrier certification (no production PQ safety claim).
#
# Cross-platform: dogego cert pq
#   go run ./cmd/dogego cert pq
#
#   .\scripts\pq_cert.ps1
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

Write-Host "=== PQ format/carrier certification (offline) ===" -ForegroundColor Cyan
go run ./cmd/dogego cert pq
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
exit 0
