# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone A: offline mainnet field evidence certification (no datadir, no RPC).
#
# Cross-platform: dogego cert field-evidence
#   go run ./cmd/dogego cert field-evidence
#
#   .\scripts\field_evidence_cert.ps1
#   .\scripts\field_evidence_cert.ps1 -RegenTestdata
param(
    [switch]$RegenTestdata,
    [switch]$TryExportAuxpow
)
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

Write-Host "=== Mainnet field evidence certification (offline) ===" -ForegroundColor Cyan

function Invoke-FieldAuxpowExport {
    param([string]$ChainDir, [switch]$Strict)
    Write-Host "`n> Exporting auxpow fixture from $ChainDir" -ForegroundColor Yellow
    $env:DOGEGO_FIELD_DATADIR = $ChainDir
    & "$PSScriptRoot\export_mainnet_field_auxpow.ps1"
    if ($LASTEXITCODE -ne 0) {
        if ($Strict) { exit $LASTEXITCODE }
        Write-Warning "auxpow export failed (continuing offline cert)"
    }
}

if ($TryExportAuxpow -or $RegenTestdata) {
    foreach ($c in @(
            (Join-Path $DogeGo "dogedata\mainnet"),
            (Join-Path (Split-Path -Parent $DogeGo) "dogedata\mainnet")
        )) {
        $auxPath = Join-Path $c "headers_aux.bin"
        if ((Test-Path $auxPath) -or (Test-Path (Join-Path $c "headers"))) {
            Invoke-FieldAuxpowExport -ChainDir $c -Strict:($TryExportAuxpow -or $RegenTestdata)
            break
        }
    }
}

if ($RegenTestdata) {
    Write-Host "`n> Regenerating consensus/testdata from canonical specs" -ForegroundColor Yellow
    Remove-Item Env:DOGEGO_FIELD_DATADIR -ErrorAction SilentlyContinue
    $env:UPDATE_CORE_TESTDATA = "1"
    go test ./consensus -run TestUpdateCoreTestdata -count=1
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if (-not $RegenTestdata) {
    foreach ($c in @(
            (Join-Path $DogeGo "dogedata\mainnet"),
            (Join-Path (Split-Path -Parent $DogeGo) "dogedata\mainnet")
        )) {
        $auxPath = Join-Path $c "headers_aux.bin"
        if (Test-Path $auxPath) {
            Write-Host "`n> Auto-export auxpow from $c" -ForegroundColor DarkGray
            Invoke-FieldAuxpowExport -ChainDir $c
            break
        }
    }
}

go run ./cmd/dogego cert field-evidence
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "`nMainnet field evidence certification passed." -ForegroundColor Green
exit 0
