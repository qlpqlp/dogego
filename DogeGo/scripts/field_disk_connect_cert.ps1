# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone A: live operator datadir bundled connect certification (requires synced dogedata).
#
#   .\scripts\field_disk_connect_cert.ps1
#   $env:DOGEGO_FIELD_DATADIR='C:\Dogedata\mainnet'; .\scripts\field_disk_connect_cert.ps1
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

Write-Host "=== Mainnet field disk connect certification ===" -ForegroundColor Cyan
Write-Host "Tip: set DOGEGO_FIELD_DISK_CONNECT_VERBOSE=1 for connect progress logs on large tiers." -ForegroundColor DarkGray
if ($env:DOGEGO_FIELD_DATADIR) {
    Write-Host "DOGEGO_FIELD_DATADIR=$($env:DOGEGO_FIELD_DATADIR)" -ForegroundColor DarkGray
} else {
    Write-Host "DOGEGO_FIELD_DATADIR not set; using default dogedata/mainnet probe paths" -ForegroundColor DarkGray
}

$tests = @(
    "go test ./consensus -run ""TestCatalogMainnetFieldBlocksProbe|TestCoreMainnetFieldDiskBundledConnect|TestCoreMainnetFieldHeaderPoW|TestMainnetFieldDiskConnectCases"" -count=1 -timeout 900s"
)

foreach ($cmd in $tests) {
    Write-Host "`n> $cmd" -ForegroundColor Yellow
    Invoke-Expression $cmd
    if ($LASTEXITCODE -ne 0) {
        Write-Host "FAILED: $cmd" -ForegroundColor Red
        exit $LASTEXITCODE
    }
}

Write-Host "`nMainnet field disk connect certification passed." -ForegroundColor Green
exit 0
