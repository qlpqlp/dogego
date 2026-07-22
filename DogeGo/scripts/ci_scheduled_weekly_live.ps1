# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone B/D/E: weekly live CI bundle on dogego-live (Core 24/24 + corruption mini).
#
# Cross-platform: dogego cert weekly-live
#   go run ./cmd/dogego cert weekly-live -mine-bootstrap -require-wallet-dat
#   go run ./cmd/dogego cert weekly-live -skip-scripts -mine-bootstrap   # preflight-only
#   go run ./cmd/dogego cert weekly-live -include-long-soak
#
# Runbook: docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md (workflow 10)
#
#   .\scripts\ci_scheduled_weekly_live.ps1
#   .\scripts\ci_scheduled_weekly_live.ps1 -IncludeLongSoak
param(
    [switch]$IncludeLongSoak,
    [switch]$MineBootstrap,
    [switch]$RequireWalletDat,
    [string]$DataDir = "dogedata",
    [string]$Network = "reboottestnet",
    [string]$DogeGoRpcPort = "44556"
)
$ErrorActionPreference = "Stop"

if ($Network -ne "reboottestnet") {
    Write-Error "Weekly live CI is reboottestnet-only."
}

Write-Host "=== CI scheduled weekly live (dogego-live) ===" -ForegroundColor Cyan

$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo
go test ./docs/... ./ui/... -count=1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$preflightArgs = @{ RequireCore = $true; DogeGoRpcPort = $DogeGoRpcPort }
if ($RequireWalletDat -or $env:DOGEGO_WALLET_DAT_REQUIRED -eq "1") { $preflightArgs.RequireWalletDat = $true }
& "$PSScriptRoot\ci_runner_preflight.ps1" @preflightArgs
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$setupArgs = @{ DogeGoRpcPort = $DogeGoRpcPort }
if ($MineBootstrap) { $setupArgs.MineBootstrap = $true }
& "$PSScriptRoot\setup_reboottestnet_core_parity.ps1" @setupArgs
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$walletDatRequired = $RequireWalletDat -or $env:DOGEGO_WALLET_DAT_REQUIRED -eq "1"
if ($walletDatRequired -or $env:DOGEGO_WALLET_DAT) {
    Write-Host "`n=== Live wallet.dat migration (RPC import) ===" -ForegroundColor Cyan
    $migrationArgs = @{ SkipOffline = $true }
    if ($walletDatRequired) { $migrationArgs.RequireWalletDat = $true }
    & "$PSScriptRoot\wallet_migration_cert.ps1" @migrationArgs
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

$env:DOGEGO_CORE_COMPARE_MIN = "24"
& "$PSScriptRoot\core_reboottestnet_core_aligned_gate.ps1" -DogeGoRpcPort $DogeGoRpcPort
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

& "$PSScriptRoot\corruption_extended_cert_mini.ps1" -Network $Network -DataDir $DataDir
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if ($IncludeLongSoak -or $env:DOGEGO_CORRUPTION_LONG_SOAK -eq "1") {
    & "$PSScriptRoot\ci_scheduled_corruption_soak.ps1" -Network $Network -DataDir $DataDir
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Write-Host "`nCI scheduled weekly live passed." -ForegroundColor Green
exit 0
