# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone A: live operator datadir field evidence (header PoW + disk connect).
# Skips gracefully when dogedata/mainnet is not synced.
#
#   .\scripts\field_evidence_live_cert.ps1
#   .\scripts\field_evidence_live_cert.ps1 -Json
#   $env:DOGEGO_FIELD_DATADIR='C:\Dogedata\mainnet'; .\scripts\field_evidence_live_cert.ps1
param(
    [switch]$Json,
    [switch]$RequireLive
)
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

if (-not $env:DOGEGO_FIELD_DATADIR) {
    foreach ($c in @(
            (Join-Path $DogeGo "dogedata\mainnet"),
            (Join-Path (Split-Path -Parent $DogeGo) "dogedata\mainnet")
        )) {
        if ((Test-Path (Join-Path $c "headers")) -or (Test-Path (Join-Path $c "headers.bin"))) {
            $env:DOGEGO_FIELD_DATADIR = $c
            break
        }
    }
}

$chainDir = if ($env:DOGEGO_FIELD_DATADIR) { $env:DOGEGO_FIELD_DATADIR } else { Join-Path (Split-Path -Parent $DogeGo) "dogedata\mainnet" }
$headersOk = (Test-Path (Join-Path $chainDir "headers")) -or (Test-Path (Join-Path $chainDir "headers.bin"))
$rawOk = Test-Path (Join-Path $chainDir "rawblocks")
$auxOk = Test-Path (Join-Path $chainDir "headers_aux.bin")

if (-not $headersOk) {
    $msg = "SKIP: no synced mainnet headers at $chainDir (offline field_evidence_cert.ps1 still passes)"
    if ($Json) {
        @{ ok = $true; live_ok = $false; skipped = $true; chain_dir = $chainDir; message = $msg } | ConvertTo-Json -Compress
    } else {
        Write-Host "=== Mainnet field evidence (live) ===" -ForegroundColor Cyan
        Write-Host $msg -ForegroundColor Yellow
    }
    if ($RequireLive) { exit 1 }
    exit 0
}

Write-Host "=== Mainnet field evidence (live) ===" -ForegroundColor Cyan
Write-Host ("chain_dir={0} rawblocks={1} aux={2}" -f $chainDir, $rawOk, $auxOk) -ForegroundColor DarkGray

& "$PSScriptRoot\field_disk_connect_cert.ps1"
$liveExit = $LASTEXITCODE
$liveOk = ($liveExit -eq 0)

if ($Json) {
    @{
        ok       = $true
        live_ok  = $liveOk
        skipped  = $false
        chain_dir = $chainDir
        has_rawblocks = $rawOk
        has_aux = $auxOk
        exit_code = $liveExit
    } | ConvertTo-Json -Compress
} elseif ($liveOk) {
    Write-Host "Live field evidence certification passed." -ForegroundColor Green
} else {
    Write-Host "Live field evidence certification failed." -ForegroundColor Red
}

if ($RequireLive -and -not $liveOk) { exit 1 }
if (-not $liveOk) { exit 2 }
exit 0
