# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: reboottestnet reindextx workflow with verifychain convergence (disruptive).
#
#   .\scripts\core_reboottestnet_reindex_workflow.ps1
param(
    [switch]$Json,
    [string]$DataDir = "dogedata",
    [string]$Network = "reboottestnet",
    [string]$DogeGoRpcPort = "44556"
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

if ($Network -ne "reboottestnet") {
    Write-Error "This workflow only runs on reboottestnet."
}

$issues = @()
$notes = @()

function Invoke-Dg {
    param([string]$Method, [object[]]$Params = @())
    return Invoke-DogeGoJsonRpc -Method $Method -Params $Params -RpcPort ([int]$DogeGoRpcPort) -WarmupRetries 5 -WarmupDelaySec 2
}

Write-Host "=== Reboottestnet reindex workflow ===" -ForegroundColor Cyan

$before = Invoke-Dg getindexinfo
$blocksBefore = (Invoke-Dg getblockchaininfo).blocks

try {
    $result = Invoke-Dg reindextx -Params @($false)
    $notes += "reindextx_blocks_indexed_$($result.blocks_indexed)"
} catch {
    $issues += "reindextx_failed"
}

Start-Sleep -Seconds 3
$after = Invoke-Dg getindexinfo
$blocksAfter = (Invoke-Dg getblockchaininfo).blocks

if ($blocksAfter -lt ($blocksBefore - 2)) {
    $issues += "blocks_regressed"
}

try {
    $vc = Invoke-Dg verifychain -Params @(2, 0)
    if ($vc -ne $true -and "$vc" -notmatch "true") { $issues += "verifychain_false" }
} catch {
    $issues += "verifychain_failed"
}

$ok = ($issues.Count -eq 0)
if ($Json) {
    [ordered]@{
        ok     = $ok
        before = $before
        after  = $after
        result = $result
        issues = @($issues)
        notes  = @($notes)
    } | ConvertTo-Json -Depth 6
} else {
    Write-Host ("blocks: {0} -> {1}" -f $blocksBefore, $blocksAfter)
    foreach ($i in $issues) { Write-Host ("FAIL: " + $i) -ForegroundColor Red }
    foreach ($n in $notes) { Write-Host ("NOTE: " + $n) -ForegroundColor DarkGray }
    if ($ok) { Write-Host "`nReboottestnet reindex workflow passed." -ForegroundColor Green }
    else { Write-Host "`nReboottestnet reindex workflow failed." -ForegroundColor Red }
}

if (-not $ok) { exit 1 }
exit 0
