# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Non-blocking connect catch-up nudge (async syncutxo RPC).
# Usage:
#   .\scripts\nudge_connect.ps1
#   .\scripts\nudge_connect.ps1 -MaxBlocks 8
param(
    [int]$MaxBlocks = 8,
    [string]$DataDir,
    [string]$Network = "mainnet"
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

if ($MaxBlocks -lt 1) { $MaxBlocks = 1 }
if ($MaxBlocks -gt 64) { $MaxBlocks = 64 }

$before = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 3
$startBlocks = [int64]$before.blocks
$beforeBoost = Format-DogeGoConnectCatchUpBoost (Get-DogeGoRpcConnectCatchUpTuning $before)
$res = Invoke-DogeGoJsonRpc -Method syncutxo -Params "[$MaxBlocks]"
Write-Host ("syncutxo: max_blocks={0} async={1}" -f $MaxBlocks, $res.dogego_syncutxo_async) -ForegroundColor Cyan
if ($beforeBoost) {
    Write-Host ("connect boost before: {0}" -f $beforeBoost) -ForegroundColor DarkGray
}
if ($res.already_in_flight) {
    Write-Host "connect replay already in flight" -ForegroundColor Yellow
}
Start-Sleep -Seconds 5
$after = Invoke-DogeGoJsonRpc -Method getblockchaininfo
$endBlocks = [int64]$after.blocks
$afterBoost = Format-DogeGoConnectCatchUpBoost (Get-DogeGoRpcConnectCatchUpTuning $after)
$line = ("blocks: {0} -> {1} (+{2}) connect_rate={3}/min lag={4}" -f $startBlocks, $endBlocks, ($endBlocks - $startBlocks), $after.dogego_connect_blocks_per_minute, $after.dogego_stored_bodies_ahead_connect)
if ($afterBoost) { $line += (" boost={0}" -f $afterBoost) }
Write-Host $line -ForegroundColor Green
