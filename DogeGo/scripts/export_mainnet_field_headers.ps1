# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Refresh consensus/testdata from local dogedata (headers + rawblocks when present).
# Writes core_header_vectors.json (field headers), mainnet_field_blocks.json, and mainnet_field_auxpow.json (when headers_aux present).
#
#   .\scripts\export_mainnet_field_headers.ps1
#   $env:DOGEGO_FIELD_DATADIR='C:\Dogedata\mainnet'; .\scripts\export_mainnet_field_headers.ps1
#   .\scripts\export_mainnet_field_headers.ps1 -CoreRpc
param(
    [string]$DataDir = "",
    [switch]$CoreRpc
)
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

if (-not $DataDir) {
    $candidates = @(
        (Join-Path $DogeGo "dogedata\mainnet"),
        (Join-Path (Split-Path -Parent $DogeGo) "dogedata\mainnet")
    )
    foreach ($c in $candidates) {
        if (Test-Path (Join-Path $c "headers")) {
            $DataDir = $c
            break
        }
    }
}
if ($DataDir) {
    $env:DOGEGO_FIELD_DATADIR = $DataDir
}
if ($CoreRpc) {
    $env:DOGEGO_ENRICH_CHECKPOINT_RPC = "1"
    if (-not $env:DOGEGO_CORE_CLI) {
        $coreCli = (Get-Command dogecoin-cli -ErrorAction SilentlyContinue).Source
        if (-not $coreCli) {
            $default = "C:\Program Files\Dogecoin\daemon\dogecoin-cli.exe"
            if (Test-Path $default) { $coreCli = $default }
        }
        if ($coreCli) { $env:DOGEGO_CORE_CLI = $coreCli }
    }
    Write-Host "Core RPC enrichment enabled (dogecoin-cli checkpoint headers)" -ForegroundColor DarkGray
}

Write-Host "Regenerating consensus/testdata (checkpoint header_hex from local headers when available)…" -ForegroundColor Cyan
$env:UPDATE_CORE_TESTDATA = "1"
go test ./consensus -run TestUpdateCoreTestdata -count=1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Done. Re-run: .\scripts\field_evidence_cert.ps1" -ForegroundColor Green
exit 0
