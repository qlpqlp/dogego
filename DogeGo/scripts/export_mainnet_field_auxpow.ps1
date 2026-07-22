# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Export mainnet_field_auxpow.json from operator dogedata (headers_aux.bin required for aux_hex).
#
#   .\scripts\export_mainnet_field_auxpow.ps1
#   $env:DOGEGO_FIELD_DATADIR='C:\Dogedata\mainnet'; .\scripts\export_mainnet_field_auxpow.ps1
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

if (-not $env:DOGEGO_FIELD_DATADIR) {
    foreach ($c in @(
            (Join-Path $DogeGo "dogedata\mainnet"),
            (Join-Path (Split-Path -Parent $DogeGo) "dogedata\mainnet")
        )) {
        if (Test-Path (Join-Path $c "headers")) {
            $env:DOGEGO_FIELD_DATADIR = $c
            break
        }
    }
}

Write-Host "=== Export mainnet_field_auxpow.json ===" -ForegroundColor Cyan
if ($env:DOGEGO_FIELD_DATADIR) {
    Write-Host "DOGEGO_FIELD_DATADIR=$($env:DOGEGO_FIELD_DATADIR)" -ForegroundColor DarkGray
} else {
    Write-Host "WARN: DOGEGO_FIELD_DATADIR not set; export keeps committed rows only" -ForegroundColor Yellow
}

$env:UPDATE_CORE_TESTDATA = "1"
go test ./consensus -run TestUpdateCoreTestdata -count=1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "`nDone. Re-run: .\scripts\field_evidence_cert.ps1" -ForegroundColor Green
exit 0
