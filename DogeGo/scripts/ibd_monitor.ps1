# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Periodic IBD monitor: health + sync snapshot + optional forward-progress window.
# Suitable for Task Scheduler / manual checks during multi-day mainnet soak.
#
#   .\scripts\ibd_monitor.ps1
#   .\scripts\ibd_monitor.ps1 -ConvergeSec 120
#   .\scripts\ibd_monitor.ps1 -Json -ConvergeSec 120
#   .\scripts\ibd_monitor.ps1 -AutoRestartOnStaleLock -ConvergeSec 120
param(
    [int]$ConvergeSec = 0,
    [switch]$Json,
    [switch]$AutoRestartOnStaleLock,
    [string]$DataDir,
    [string]$Network = "mainnet"
)
$ErrorActionPreference = "Stop"
$ts = (Get-Date).ToString("o")

if ($AutoRestartOnStaleLock) {
    . "$PSScriptRoot\dogego_rpc.ps1"
    if (-not (Get-Process dogego -ErrorAction SilentlyContinue)) {
        if (Remove-DogeGoStaleProcessLock -DataDir $DataDir -Network $Network) {
            Write-Host "Stale process lock - resuming node..." -ForegroundColor Yellow
            & "$PSScriptRoot\resume_node.ps1" -WaitSec 20
        }
    }
}

if ($Json) {
    . "$PSScriptRoot\dogego_rpc.ps1"
    $healthJson = & "$PSScriptRoot\node_health.ps1" -Json -DataDir $DataDir -Network $Network
    $healthExit = $LASTEXITCODE
    $health = $healthJson | ConvertFrom-Json
    $out = [ordered]@{
        timestamp = $ts
        health    = $health
    }
    try {
        $ibdInfo = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 2 -WarmupDelaySec 2 -TimeoutSec 30
        $out.body_ibd = Get-DogeGoBodyIBDSnapshot $ibdInfo
    } catch { }
    if ($ConvergeSec -gt 0) {
        $convParams = @{
            IntervalSec        = $ConvergeSec
            MinBlocksAdvance   = 1
            MinContiguousAdvance = 1
            MinRawProbeAdvance = 1
            Network            = $Network
        }
        if ($DataDir) { $convParams.DataDir = $DataDir }
        & "$PSScriptRoot\ibd_convergence_check.ps1" @convParams | Out-Null
        $out.convergence_ok = ($LASTEXITCODE -eq 0)
        $out.convergence_exit = $LASTEXITCODE
    }
    $out | ConvertTo-Json -Compress -Depth 6
    if ($healthExit -ge 2) { exit 2 }
    if ($ConvergeSec -gt 0 -and $out.convergence_ok -ne $true) { exit 1 }
    exit 0
}

Write-Host "=== DogeGo IBD monitor ($ts) ===" -ForegroundColor Cyan
& "$PSScriptRoot\node_health.ps1" -DataDir $DataDir -Network $Network
$healthExit = $LASTEXITCODE
Write-Host ""
& "$PSScriptRoot\sync_status.ps1" -DataDir $DataDir -Network $Network

try {
    . "$PSScriptRoot\dogego_rpc.ps1"
    $ibdInfo = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 2 -WarmupDelaySec 2 -TimeoutSec 30
    $bodySnap = Get-DogeGoBodyIBDSnapshot $ibdInfo
    $parts = @()
    if ($bodySnap.download_per_min) { $parts += ("dl={0}/min" -f $bodySnap.download_per_min) }
    if ($null -ne $bodySnap.in_flight) { $parts += "in_flight=$($bodySnap.in_flight)" }
    if ($null -ne $bodySnap.assist_sessions) { $parts += "assist=$($bodySnap.assist_sessions)" }
    if ($null -ne $bodySnap.assist_pool) { $parts += "pool=$($bodySnap.assist_pool)" }
    if ($null -ne $bodySnap.last_body_store_min) { $parts += "last_store=$($bodySnap.last_body_store_min)m" }
    if ($bodySnap.body_eta_text) { $parts += "eta=$($bodySnap.body_eta_text)" }
    if ($bodySnap.body_pct) { $parts += "body_pct=$($bodySnap.body_pct)" }
    if ($bodySnap.header_paused -eq $true) { $parts += "hdr_paused=1" }
    if ($null -ne $bodySnap.header_resume_blocks) {
        $hr = "hdr_resume=$($bodySnap.header_resume_blocks)"
        if ($bodySnap.header_resume_eta_text) { $hr += " (~$($bodySnap.header_resume_eta_text))" }
        $parts += $hr
    }
    if ($parts.Count -gt 0) {
        Write-Host ("Body IBD pump: {0}" -f ($parts -join " ")) -ForegroundColor DarkGray
    }
} catch { }

if ($ConvergeSec -gt 0) {
    Write-Host ""
    $convParams = @{
        IntervalSec          = $ConvergeSec
        MinBlocksAdvance     = 1
        MinContiguousAdvance = 1
        MinRawProbeAdvance   = 1
        Network              = $Network
    }
    if ($DataDir) { $convParams.DataDir = $DataDir }
    & "$PSScriptRoot\ibd_convergence_check.ps1" @convParams
    if ($LASTEXITCODE -ne 0) { exit 1 }
}

if ($healthExit -ge 2) { exit 2 }
exit 0
