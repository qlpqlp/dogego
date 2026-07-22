# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# DogeGo offline CI gate (Windows PowerShell). No live node required.
#
# Cross-platform: dogego cert offline
#   go run ./cmd/dogego cert offline
#
#   .\scripts\ci_offline_gate.ps1
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

Write-Host "=== DogeGo offline CI gate ===" -ForegroundColor Cyan
go run ./cmd/dogego cert offline
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "`nOffline CI gate passed." -ForegroundColor Green
exit 0
