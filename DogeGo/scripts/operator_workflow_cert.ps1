# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# DogeGo standalone operator certification (offline, no P2P).
#
# Cross-platform: dogego cert operator
#   go run ./cmd/dogego cert operator
#
#   .\scripts\operator_workflow_cert.ps1
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

Write-Host "=== DogeGo operator workflow certification ===" -ForegroundColor Cyan
go run ./cmd/dogego cert operator
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if ($env:DOGEGO_FIELD_DISK_CONNECT -eq "1") {
    Write-Host "`n> mainnet field disk connect cert (live datadir)" -ForegroundColor DarkGray
    & "$PSScriptRoot\field_disk_connect_cert.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

exit 0
