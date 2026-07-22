# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E (partial): restart DogeGo on mainnet and compare chain state vs Core before/after.
# Disruptive for DogeGo only - Core is not restarted.
#
#   .\scripts\core_mainnet_restart_compare.ps1 -AllowMainnet
param(
    [switch]$AllowMainnet,
    [switch]$Json,
    [string]$DataDir = "dogedata",
    [string]$Network = "mainnet",
    [int]$WaitSec = 60,
    [string]$DogeGoRpcPort = "22557",
    [string]$CoreRpcPort = "22555"
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

if ($Network -eq "mainnet" -and -not $AllowMainnet) {
    Write-Error "Mainnet restart compare requires -AllowMainnet."
}

$coreCli = $env:DOGEGO_CORE_CLI
if (-not $coreCli) {
    $coreCli = (Get-Command dogecoin-cli -ErrorAction SilentlyContinue).Source
}
if (-not $coreCli) {
    $d = "C:\Program Files\Dogecoin\daemon\dogecoin-cli.exe"
    if (Test-Path $d) { $coreCli = $d }
}

function Snapshot-DogeGo {
    return Invoke-DogeGoJsonRpc -Method getblockchaininfo -RpcPort ([int]$DogeGoRpcPort) -WarmupRetries 5 -WarmupDelaySec 2
}

function Snapshot-Core {
    if (-not $coreCli) { return $null }
    $args = @("-rpcport=$CoreRpcPort", "getblockchaininfo")
    if ($env:DOGEGO_CORE_RPC_USER) {
        $args = @("-rpcuser=$env:DOGEGO_CORE_RPC_USER", "-rpcpassword=$env:DOGEGO_CORE_RPC_PASS") + $args
    }
    $out = & $coreCli @args 2>&1
    if ($LASTEXITCODE -ne 0) { return $null }
    return $out | ConvertFrom-Json
}

$issues = @()
$notes = @()
$warnings = @()

Write-Host "=== Mainnet restart compare (DogeGo restart, Core read-only) ===" -ForegroundColor Cyan

$dgBefore = Snapshot-DogeGo
$coreBefore = Snapshot-Core
if ($coreBefore) {
    $notes += "core_before_headers_$($coreBefore.headers)"
}

& "$PSScriptRoot\core_restart_workflow.ps1" -Network $Network -DataDir $DataDir -WaitSec $WaitSec
if ($LASTEXITCODE -ne 0) {
    $issues += "restart_workflow_failed"
}

$dgAfter = Snapshot-DogeGo
$coreAfter = Snapshot-Core

if ($dgBefore -and $dgAfter) {
    if ([int64]$dgAfter.headers -lt [int64]$dgBefore.headers - 1000) {
        $issues += "dogego_headers_regressed"
    }
    if ([int64]$dgAfter.blocks -lt [int64]$dgBefore.blocks - 64) {
        $issues += "dogego_blocks_regressed"
    }
    $notes += "dogego_header_delta_$([int64]$dgAfter.headers - [int64]$dgBefore.headers)"
    $notes += "dogego_block_delta_$([int64]$dgAfter.blocks - [int64]$dgBefore.blocks)"
}

if ($coreBefore -and $coreAfter) {
    $hDelta = [Math]::Abs([int64]$coreAfter.headers - [int64]$coreBefore.headers)
    if ($hDelta -gt 100) { $notes += "core_headers_moved_$hDelta" }
    if ($dgAfter -and $coreAfter) {
        $align = [Math]::Abs([int64]$dgAfter.headers - [int64]$coreAfter.headers)
        $notes += "post_restart_header_align_delta_$align"
        if ($align -gt 600000) { $warnings += "header_align_delta_large" }
    }
}

try {
    $vc = Invoke-DogeGoJsonRpc -Method verifychain -Params @(2, 0) -RpcPort ([int]$DogeGoRpcPort) -WarmupRetries 3
    if ($vc -ne $true -and "$vc" -notmatch "true") { $issues += "verifychain_false" }
} catch {
    $issues += "verifychain_failed"
}

$ok = ($issues.Count -eq 0)
$report = [ordered]@{
    ok         = $ok
    network    = $Network
    dogego     = @{ before = $dgBefore; after = $dgAfter }
    core       = @{ before = $coreBefore; after = $coreAfter }
    issues     = @($issues)
    notes      = @($notes)
}

if ($Json) {
    $report | ConvertTo-Json -Depth 6
} else {
    if ($dgBefore -and $dgAfter) {
        Write-Host ("DogeGo headers: {0} -> {1}" -f $dgBefore.headers, $dgAfter.headers)
        Write-Host ("DogeGo blocks:  {0} -> {1}" -f $dgBefore.blocks, $dgAfter.blocks)
    }
    foreach ($i in $issues) { Write-Host ("FAIL: " + $i) -ForegroundColor Red }
    foreach ($n in $notes) { Write-Host ("NOTE: " + $n) -ForegroundColor DarkGray }
    if ($ok) { Write-Host "`nMainnet restart compare passed." -ForegroundColor Green }
    else { Write-Host "`nMainnet restart compare failed." -ForegroundColor Red }
}

if (-not $ok) { exit 1 }
exit 0
