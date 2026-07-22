# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: mainnet side-by-side operator runbook (non-disruptive).
# Requires Dogecoin Core on :22555 and DogeGo on :22557 (or use -SkipStart when DogeGo already runs).
# No corruption inject or restart workflow on mainnet.
#
#   .\scripts\core_mainnet_side_by_side_runbook.ps1 -AllowMainnet
#   .\scripts\core_mainnet_side_by_side_runbook.ps1 -AllowMainnet -SkipStart -SkipOffline
param(
    [switch]$AllowMainnet,
    [switch]$SkipOffline,
    [switch]$SkipStart,
    [string]$DataDir = "dogedata",
    [string]$DogeGoRpcPort = "22557",
    [string]$CoreRpcPort = "22555",
    [int]$WaitSec = 45
)
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

if (-not $AllowMainnet) {
    Write-Error "Mainnet runbook requires -AllowMainnet."
}

Write-Host "=== Mainnet Core vs DogeGo side-by-side runbook ===" -ForegroundColor Cyan
Write-Host "Core RPC :$CoreRpcPort  DogeGo RPC :$DogeGoRpcPort  DataDir: $DataDir" -ForegroundColor DarkGray

$steps = @()
$failed = $false

function Step($Name, $Script, $Args) {
    Write-Host "`n--- $Name ---" -ForegroundColor Yellow
    & $Script @Args
    $ok = ($LASTEXITCODE -eq 0)
    $script:steps += [ordered]@{ name = $Name; ok = $ok }
    if (-not $ok) { $script:failed = $true }
    return $ok
}

if (-not $SkipOffline) {
    Step "field_evidence_offline" "$PSScriptRoot\field_evidence_cert.ps1" @()
    if (-not $failed) {
        Step "mempool_corpus_offline" "$PSScriptRoot\core_mempool_corpus_probe.ps1" @()
        Step "bip125_offline" "$PSScriptRoot\core_mempool_bip125_offline_probe.ps1" @()
    }
}

$env:DOGEGO_CORE_COMPARE = "1"
$env:DOGEGO_MEMPOOL_PROBE = "1"
$env:DOGEGO_MAINTENANCE_PROBE = "1"
$env:DOGEGO_RESTART_RESUME = "1"
$env:DOGEGO_REINDEX_PROBE = "1"
$env:DOGEGO_BIP152_PROBE = "1"
$env:DOGEGO_RPC_PORT = $DogeGoRpcPort
$env:DOGEGO_CORE_RPC_PORT = $CoreRpcPort

$compareArgs = @{
    DataDir       = $DataDir
    Network       = "mainnet"
    DogeGoRpcPort = $DogeGoRpcPort
    CoreRpcPort   = $CoreRpcPort
    WaitSec       = $WaitSec
    MempoolProbe  = $true
}
if ($SkipStart) { $compareArgs.SkipStart = $true }

if (-not $failed) {
    Step "core_compare_with_core" "$PSScriptRoot\core_compare_with_core.ps1" @compareArgs
}

if (-not $failed) {
    Step "core_maintenance_compare" "$PSScriptRoot\core_mainnet_maintenance_compare.ps1" @{}
}

if (-not $failed) {
    Step "core_reindex_compare" "$PSScriptRoot\core_mainnet_reindex_compare.ps1" @{}
}

if (-not $failed) {
    Step "core_end_to_end_mainnet" "$PSScriptRoot\core_end_to_end_workflow.ps1" @{ Network = "mainnet"; DataDir = $DataDir }
}

if (-not $failed) {
    $blkPath = Join-Path $DataDir "mainnet\blk00000.dat"
    if (-not (Test-Path $blkPath)) {
        $blkPath = Join-Path (Split-Path -Parent (Split-Path -Parent $PSScriptRoot)) "dogedata\mainnet\blk00000.dat"
    }
    if ((Test-Path $blkPath) -or $env:DOGEGO_FIELD_DISK_CONNECT -eq "1") {
        Step "field_disk_connect" "$PSScriptRoot\field_disk_connect_cert.ps1" @()
    } else {
        Write-Host "Skipping field disk connect (no blk00000.dat; set DOGEGO_FIELD_DISK_CONNECT=1)" -ForegroundColor DarkGray
    }
}

if ($failed) {
    Write-Host "`nMainnet side-by-side runbook failed." -ForegroundColor Red
    foreach ($s in $steps) {
        if (-not $s.ok) { Write-Host ("  FAIL: " + $s.name) -ForegroundColor Red }
    }
    exit 1
}

Write-Host "`nMainnet side-by-side runbook passed." -ForegroundColor Green
exit 0
