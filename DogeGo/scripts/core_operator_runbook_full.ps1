# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: full Core-equivalent operator runbook (offline + live probes).
# Bundles all DOGEGO_* certification flags into one entry point.
#
# Offline only (CI-safe):
#   .\scripts\core_operator_runbook_full.ps1 -OfflineOnly
#
# Reboot testnet live cert (node running; includes disruptive corruption inject):
#   .\scripts\core_operator_runbook_full.ps1
#
# Mainnet side-by-side vs dogecoin-cli (Core + DogeGo both required):
#   .\scripts\core_operator_runbook_full.ps1 -Mainnet -AllowMainnet -CoreCompare
#
# Mainnet-only side-by-side runbook (offline gates + live compare, non-disruptive):
#   .\scripts\core_mainnet_side_by_side_runbook.ps1 -AllowMainnet
param(
    [switch]$OfflineOnly,
    [switch]$Mainnet,
    [switch]$AllowMainnet,
    [switch]$CoreCompare,
    [string]$DataDir = "dogedata",
    [string]$Network = "reboottestnet"
)
$ErrorActionPreference = "Stop"

if ($Mainnet) {
    $Network = "mainnet"
    if (-not $AllowMainnet) {
        Write-Error "Mainnet live probes require -AllowMainnet."
    }
}

Write-Host "=== Core operator runbook (full) ===" -ForegroundColor Cyan
Write-Host "Network: $Network  DataDir: $DataDir  OfflineOnly: $OfflineOnly" -ForegroundColor DarkGray

$env:DOGEGO_CORRUPTION_SOAK = "1"

if ($OfflineOnly) {
    & "$PSScriptRoot\core_operator_workflow_cert.ps1"
    exit $LASTEXITCODE
}

$env:DOGEGO_IBD_SOAK = "1"
$env:DOGEGO_IBD_CONVERGE = "1"
$env:DOGEGO_RESTART_RESUME = "1"
$env:DOGEGO_RESTART_CONNECT_CHECK = "1"
$env:DOGEGO_MAINTENANCE_PROBE = "1"
$env:DOGEGO_REINDEX_PROBE = "1"
$env:DOGEGO_BIP152_PROBE = "1"
$env:DOGEGO_MEMPOOL_PROBE = "1"
$env:DOGEGO_TIMED_SOAK = "1"

if ($CoreCompare -or $Mainnet) {
    $env:DOGEGO_CORE_COMPARE = "1"
}

if ($Network -ne "mainnet") {
    $env:DOGEGO_CORRUPTION_INJECT_SOAK = "1"
    $env:DOGEGO_RESTART_WORKFLOW = "1"
    $env:DOGEGO_MEMPOOL_STATEFUL_PROBE = "1"
    $env:DOGEGO_REBOOTTESTNET_REINDEX = "1"
} else {
    Write-Host "Skipping disruptive corruption/restart inject on mainnet (use reboottestnet for inject soak)." -ForegroundColor DarkGray
}

& "$PSScriptRoot\core_operator_workflow_cert.ps1"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if ($env:DOGEGO_CORE_COMPARE -eq "1") {
    Write-Host "`n[end-to-end] Bundled workflow probe" -ForegroundColor Yellow
    $e2eArgs = @{ Network = $Network }
    if ($DataDir) { $e2eArgs.DataDir = $DataDir }
    & "$PSScriptRoot\core_end_to_end_workflow.ps1" @e2eArgs
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($Network -ne "mainnet") {
    Write-Host "`n[e2e-full] Reboottestnet full operator runbook" -ForegroundColor Yellow
    $fullArgs = @{ Network = $Network; DataDir = $DataDir }
    if ($CoreCompare) { $fullArgs.IncludeCoreCompare = $true }
    & "$PSScriptRoot\core_e2e_full_runbook.ps1" @fullArgs
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Write-Host "`nCore operator runbook (full) passed." -ForegroundColor Green
exit 0
