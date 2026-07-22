# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: side-by-side maintenance RPC compare (Core :22555 vs DogeGo :22557).
# Read-only - no reindex/prune/restart.
#
#   .\scripts\core_mainnet_maintenance_compare.ps1
#   .\scripts\core_mainnet_maintenance_compare.ps1 -Json
param(
    [switch]$Json,
    [string]$DogeGoRpcPort = "22557",
    [string]$CoreRpcPort = "22555"
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

$coreCli = $env:DOGEGO_CORE_CLI
if (-not $coreCli) {
    $coreCli = (Get-Command dogecoin-cli -ErrorAction SilentlyContinue).Source
}
if (-not $coreCli) {
    $d = "C:\Program Files\Dogecoin\daemon\dogecoin-cli.exe"
    if (Test-Path $d) { $coreCli = $d }
}

$issues = @()
$warnings = @()
$notes = @()

function Invoke-Dg($method, $params = @()) {
    return Invoke-DogeGoJsonRpc -Method $method -Params $params -RpcPort ([int]$DogeGoRpcPort) -WarmupRetries 5 -WarmupDelaySec 2
}

function Invoke-Core($method, $params = @()) {
    if (-not $coreCli) { return $null }
    $args = @("-rpcport=$CoreRpcPort", $method) + $params
    if ($env:DOGEGO_CORE_RPC_USER) {
        $args = @("-rpcuser=$env:DOGEGO_CORE_RPC_USER", "-rpcpassword=$env:DOGEGO_CORE_RPC_PASS") + $args
    }
    $out = & $coreCli @args 2>&1
    if ($LASTEXITCODE -ne 0) { throw ($out | Out-String) }
    return $out | ConvertFrom-Json
}

$dgInfo = $null
$coreInfo = $null
try { $dgInfo = Invoke-Dg getblockchaininfo } catch { $issues += "dogego_getblockchaininfo_failed" }
if ($coreCli) {
    try { $coreInfo = Invoke-Core getblockchaininfo } catch { $warnings += "core_getblockchaininfo_skipped" }
}

if ($dgInfo -and $coreInfo) {
    if ($dgInfo.chain -ne $coreInfo.chain) { $issues += "chain_name_mismatch" }
    $hDelta = [Math]::Abs([int64]$dgInfo.headers - [int64]$coreInfo.headers)
    $bDelta = [Math]::Abs([int64]$dgInfo.blocks - [int64]$coreInfo.blocks)
    $notes += "header_delta_$hDelta"
    $notes += "block_delta_$bDelta"
    if ($hDelta -gt 600000) { $warnings += "header_delta_large" }
}

foreach ($level in @(2, 4)) {
    try {
        $dgV = Invoke-Dg verifychain -Params @($level, 0)
        if ($dgV -ne $true -and "$dgV" -notmatch "true") {
            if ($dgInfo.initialblockdownload) { $warnings += "dogego_verifychain_${level}_ibd" }
            else { $issues += "dogego_verifychain_${level}_false" }
        }
    } catch { $issues += "dogego_verifychain_${level}_failed" }
    if ($coreCli -and $coreInfo) {
        try {
            $coreV = Invoke-Core verifychain @($level, 0)
            if ($coreV -ne $true -and "$coreV" -notmatch "true") {
                $warnings += "core_verifychain_${level}_not_true"
            } else {
                $notes += "verifychain_${level}_core_ok"
            }
        } catch { $warnings += "core_verifychain_${level}_skipped" }
    }
}

$dgIdx = $null
$coreIdx = $null
try { $dgIdx = Invoke-Dg getindexinfo } catch { $issues += "dogego_getindexinfo_failed" }
if ($coreCli) {
    try { $coreIdx = Invoke-Core getindexinfo } catch { $warnings += "core_getindexinfo_skipped" }
}
if ($dgIdx -and $coreIdx) {
    foreach ($key in @("txindex", "basic")) {
        $dgHas = $dgIdx.PSObject.Properties.Name -contains $key
        $coreHas = $coreIdx.PSObject.Properties.Name -contains $key
        if ($dgHas -ne $coreHas) { $warnings += "getindexinfo_${key}_presence_mismatch" }
    }
}

$dgStats = $null
$coreStats = $null
try { $dgStats = Invoke-Dg getchaintxstats -Params @(24) } catch { $warnings += "dogego_getchaintxstats_failed" }
if ($coreCli) {
    try { $coreStats = Invoke-Core getchaintxstats @("24") } catch { $warnings += "core_getchaintxstats_skipped" }
}
if ($dgStats -and $coreStats) {
    $dgWin = if ($dgStats.PSObject.Properties.Name -contains "window_tx_count") { [int64]$dgStats.window_tx_count } else { [int64]$dgStats.txcount }
    $coreWin = if ($coreStats.PSObject.Properties.Name -contains "window_tx_count") { [int64]$coreStats.window_tx_count } else { [int64]$coreStats.txcount }
    $delta = [Math]::Abs($dgWin - $coreWin)
    $notes += "chaintxstats_window_delta_$delta"
    if ($delta -gt 500 -and -not $dgInfo.initialblockdownload) { $warnings += "chaintxstats_window_misaligned" }
}

$ok = ($issues.Count -eq 0)
$report = [ordered]@{
    ok       = $ok
    dogego   = @{ blocks = $dgInfo.blocks; headers = $dgInfo.headers; ibd = $dgInfo.initialblockdownload }
    core     = if ($coreInfo) { @{ blocks = $coreInfo.blocks; headers = $coreInfo.headers; ibd = $coreInfo.initialblockdownload } } else { $null }
    issues   = @($issues)
    warnings = @($warnings)
    notes    = @($notes)
}

if ($Json) {
    $report | ConvertTo-Json -Depth 6
} else {
    Write-Host "=== Mainnet maintenance compare (Core vs DogeGo) ===" -ForegroundColor Cyan
    if ($dgInfo) {
        Write-Host ("DogeGo: blocks={0} headers={1} ibd={2}" -f $dgInfo.blocks, $dgInfo.headers, $dgInfo.initialblockdownload)
    }
    if ($coreInfo) {
        Write-Host ("Core:   blocks={0} headers={1} ibd={2}" -f $coreInfo.blocks, $coreInfo.headers, $coreInfo.initialblockdownload)
    } elseif (-not $coreCli) {
        Write-Host "Core CLI not found - DogeGo-only maintenance checks." -ForegroundColor Yellow
    }
    foreach ($w in $warnings) { Write-Host ("WARN: " + $w) -ForegroundColor Yellow }
    foreach ($i in $issues) { Write-Host ("FAIL: " + $i) -ForegroundColor Red }
    foreach ($n in $notes) { Write-Host ("NOTE: " + $n) -ForegroundColor DarkGray }
    if ($ok) { Write-Host "`nMaintenance compare passed." -ForegroundColor Green }
    else { Write-Host "`nMaintenance compare failed." -ForegroundColor Red }
}

if (-not $ok) { exit 1 }
exit 0
