# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: disruptive reindex / optional prune with verifychain + Core index compare.
# Default: reboottestnet. Mainnet requires -AllowMainnet -ConfirmDisruptive.
#
#   .\scripts\core_reindex_prune_disruptive_workflow.ps1
#   .\scripts\core_reindex_prune_disruptive_workflow.ps1 -IncludeBlockFilters -IncludePrune
#   .\scripts\core_reindex_prune_disruptive_workflow.ps1 -AllowMainnet -ConfirmDisruptive -IncludeCoreCompare
param(
    [switch]$Json,
    [switch]$AllowMainnet,
    [switch]$ConfirmDisruptive,
    [switch]$IncludeBlockFilters,
    [switch]$IncludePrune,
    [switch]$IncludeCoreCompare,
    [string]$DataDir = "dogedata",
    [string]$Network = "reboottestnet",
    [string]$DogeGoRpcPort = "",
    [string]$CoreRpcPort = "",
    [int]$PruneKeepBlocks = 80
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

if ($Network -eq "mainnet") {
    if (-not $AllowMainnet -or -not $ConfirmDisruptive) {
        Write-Error "Mainnet disruptive reindex/prune requires -AllowMainnet -ConfirmDisruptive."
    }
    if ($IncludePrune) {
        Write-Error "IncludePrune is reboottestnet-only (mainnet prune is operator-local only)."
    }
}

if ($Network -eq "reboottestnet") {
    if (-not $DogeGoRpcPort) { $DogeGoRpcPort = "44556" }
    if (-not $CoreRpcPort) { $CoreRpcPort = "44555" }
} else {
    if (-not $DogeGoRpcPort) { $DogeGoRpcPort = "22557" }
    if (-not $CoreRpcPort) { $CoreRpcPort = "22555" }
}

$rpcPort = [int]$DogeGoRpcPort
$issues = @()
$warnings = @()
$notes = @()
$steps = @()

function Invoke-Dg {
    param([string]$Method, [object[]]$Params = @())
    return Invoke-DogeGoJsonRpc -Method $Method -Params $Params -RpcPort $rpcPort -WarmupRetries 5 -WarmupDelaySec 2
}

function Step {
    param([string]$Name, [scriptblock]$Body)
    Write-Host "`n--- $Name ---" -ForegroundColor Cyan
    $ok = $true
    try {
        & $Body
    } catch {
        $ok = $false
        $script:issues += "${Name}_failed"
        Write-Host ("FAIL: " + $_) -ForegroundColor Red
    }
    $script:steps += [ordered]@{ name = $Name; ok = $ok }
    return $ok
}

Write-Host "=== Disruptive reindex/prune workflow ($Network) ===" -ForegroundColor Cyan

$beforeIdx = $null
$blocksBefore = $null
Step "snapshot_before" {
    $script:beforeIdx = Invoke-Dg getindexinfo
    $script:blocksBefore = (Invoke-Dg getblockchaininfo).blocks
    $script:notes += "blocks_before=$($script:blocksBefore)"
}

Step "reindextx" {
    $res = Invoke-Dg reindextx -Params @($false)
    $script:notes += "reindextx_blocks_indexed=$($res.blocks_indexed)"
}

if ($IncludeBlockFilters) {
    Step "reindexblockfilters" {
        $res = Invoke-Dg reindexblockfilters
        if ($res.blocks_indexed -lt 0) { throw "unexpected blocks_indexed" }
        $script:notes += "reindexblockfilters_blocks=$($res.blocks_indexed)"
    }
}

if ($IncludePrune) {
    if ($Network -ne "reboottestnet") {
        throw "IncludePrune only on reboottestnet"
    }
    Step "pruneblockchain" {
        $h = [int64](Invoke-Dg getblockchaininfo).blocks
        $target = [Math]::Max(1, $h - $PruneKeepBlocks)
        $pruned = Invoke-Dg pruneblockchain -Params @($target)
        $script:notes += "pruneblockchain_height=$pruned"
    }
}

Step "verifychain" {
    $vc = Invoke-Dg verifychain -Params @(2, 0)
    if ($vc -ne $true -and "$vc" -notmatch "true") { throw "verifychain returned false" }
}

$afterIdx = $null
$blocksAfter = $null
Step "snapshot_after" {
    $script:afterIdx = Invoke-Dg getindexinfo
    $script:blocksAfter = (Invoke-Dg getblockchaininfo).blocks
    $script:notes += "blocks_after=$($script:blocksAfter)"
}

if ($blocksBefore -ne $null -and $blocksAfter -lt ($blocksBefore - 3)) {
    $issues += "blocks_regressed"
}

if ($IncludeCoreCompare -or $env:DOGEGO_CORE_COMPARE -eq "1") {
    $env:DOGEGO_CORE_COMPARE = "1"
    $env:DOGEGO_CORE_RPC_PORT = $CoreRpcPort
    Step "core_index_compare" {
        if ($Network -eq "mainnet") {
            & "$PSScriptRoot\core_mainnet_reindex_compare.ps1" -DogeGoRpcPort $DogeGoRpcPort -CoreRpcPort $CoreRpcPort
        } else {
            & "$PSScriptRoot\core_reboottestnet_reindex_compare.ps1" -DogeGoRpcPort $DogeGoRpcPort -CoreRpcPort $CoreRpcPort
        }
        if ($LASTEXITCODE -ne 0) { throw "core index compare failed" }
    }
}

$ok = ($issues.Count -eq 0)
$report = [ordered]@{
    ok       = $ok
    network  = $Network
    steps    = $steps
    before   = $beforeIdx
    after    = $afterIdx
    issues   = @($issues)
    warnings = @($warnings)
    notes    = @($notes)
}

if ($Json) {
    $report | ConvertTo-Json -Depth 6
} else {
    foreach ($w in $warnings) { Write-Host ("WARN: " + $w) -ForegroundColor Yellow }
    foreach ($i in $issues) { Write-Host ("FAIL: " + $i) -ForegroundColor Red }
    foreach ($n in $notes) { Write-Host ("NOTE: " + $n) -ForegroundColor DarkGray }
    if ($ok) { Write-Host "`nDisruptive reindex/prune workflow passed." -ForegroundColor Green }
    else { Write-Host "`nDisruptive reindex/prune workflow failed." -ForegroundColor Red }
}

if (-not $ok) { exit 1 }
exit 0
