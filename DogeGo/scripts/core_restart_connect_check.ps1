# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: after restart, verify chainActive catches up to stored bodies (Core resume invariant).
#
#   .\scripts\core_restart_connect_check.ps1
#   .\scripts\core_restart_connect_check.ps1 -MaxLag 256 -TimeoutSec 600
param(
    [int64]$MaxLag = 128,
    [int]$TimeoutSec = 300,
    [int]$PollSec = 15,
    [int]$RpcTimeoutSec = 90,
    [string]$DataDir,
    [string]$Network = "mainnet"
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

$deadline = (Get-Date).AddSeconds($TimeoutSec)
Write-Host "=== Post-restart connect catch-up check (max lag $MaxLag, ${TimeoutSec}s) ===" -ForegroundColor Cyan

while ((Get-Date) -lt $deadline) {
    $snap = Get-DogeGoSyncHeightsSnapshot -RpcTimeoutSec $RpcTimeoutSec -RpcWarmupRetries 2
    $blocks = $snap.blocks
    $cont = $snap.stored
    $lag = $snap.lag
    $src = if ($snap.source) { $snap.source } else { "?" }
    $lagStr = if ($null -ne $lag) { $lag } else { "?" }
    $boostLine = $null
    try {
        $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 1 -TimeoutSec $RpcTimeoutSec
        $boostLine = Format-DogeGoConnectCatchUpBoost (Get-DogeGoRpcConnectCatchUpTuning $info)
    } catch { }
    $line = ("{0}: blocks={1} stored={2} lag={3} ({4})" -f (Get-Date -Format "HH:mm:ss"), $blocks, $cont, $lagStr, $src)
    if ($boostLine) { $line += " boost=$boostLine" }
    Write-Host $line
    if ($null -ne $lag -and $lag -le $MaxLag) {
        Write-Host "OK: connect caught up (lag <= $MaxLag)." -ForegroundColor Green
        exit 0
    }
    Start-Sleep -Seconds $PollSec
}

Write-Host "FAIL: connect lag still above $MaxLag after ${TimeoutSec}s" -ForegroundColor Red
exit 1
