# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Poll DogeGo sync progress (RPC and optional web /api/summary).
# Usage:
#   .\scripts\watch_sync.ps1
#   .\scripts\watch_sync.ps1 -IntervalSec 30 -WebUI http://127.0.0.1:2013
param(
    [int]$IntervalSec = 30,
    [string]$WebUI = "http://127.0.0.1:2013",
    [string]$RpcUser,
    [string]$RpcPassword,
    [string]$RpcHost,
    [int]$RpcPort = 0
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"
$rpcParams = @{
    WarmupRetries  = 5
    WarmupDelaySec = 2
}
if ($RpcUser) { $rpcParams.RpcUser = $RpcUser }
if ($RpcPassword) { $rpcParams.RpcPassword = $RpcPassword }
if ($RpcHost) { $rpcParams.RpcHost = $RpcHost }
if ($RpcPort -gt 0) { $rpcParams.RpcPort = $RpcPort }

function Get-Summary {
    try {
        $r = Invoke-WebRequest -Uri ($WebUI.TrimEnd('/') + "/api/summary") -UseBasicParsing -TimeoutSec 5
        return $r.Content | ConvertFrom-Json
    } catch {
        return $null
    }
}

Write-Host "Watching DogeGo sync every ${IntervalSec}s (Ctrl+C to stop)..." -ForegroundColor Cyan
$script:LastBlocks = $null
while ($true) {
    $ts = Get-Date -Format "HH:mm:ss"
    try {
        $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo @rpcParams
    } catch {
        Write-Host "[$ts] RPC unavailable: $_" -ForegroundColor Yellow
        $info = $null
    }
    if ($info) {
        $line = "[$ts] headers=$($info.headers) blocks=$($info.blocks) ibd=$($info.initialblockdownload)"
        if ($info.PSObject.Properties.Name -contains "dogego_contiguous_raw_height") {
            $line += " stored=$($info.dogego_contiguous_raw_height)"
        }
        if ($info.PSObject.Properties.Name -contains "dogego_raw_sync" -and $info.dogego_raw_sync.next_probe_height) {
            $line += " probe=$($info.dogego_raw_sync.next_probe_height)"
        }
        $cl = Get-DogeGoRpcConnectLag $info
        if ($null -ne $cl -and $cl -gt 0) {
            $line += " connect_lag=$cl"
        }
        $cRate = Get-DogeGoRpcConnectBlocksPerMinute $info
        if ($null -ne $cRate -and $cRate -gt 0) {
            $line += (" connect={0:N1}/min" -f $cRate)
        }
        $boostLine = Format-DogeGoConnectCatchUpBoost (Get-DogeGoRpcConnectCatchUpTuning $info)
        if ($boostLine) { $line += " boost=$boostLine" }
        if ($null -ne $script:LastBlocks -and $info.blocks -ne $script:LastBlocks) {
            $bd = [int64]$info.blocks - [int64]$script:LastBlocks
            if ($bd -gt 0) { $line += " connect_delta=$bd" }
        }
        $script:LastBlocks = $info.blocks
        if ($info.PSObject.Properties.Name -contains "dogego_raw_sync" -and $info.dogego_raw_sync.blocks_per_minute) {
            $line += (" rate={0:N1}/min" -f [double]$info.dogego_raw_sync.blocks_per_minute)
            if ($info.PSObject.Properties.Name -contains "dogego_contiguous_raw_height") {
                $eta = Format-DogeGoBodyIBDEta -HeaderTip ([int64]$info.headers) -Contiguous ([int64]$info.dogego_contiguous_raw_height) -BlocksPerMinute ([double]$info.dogego_raw_sync.blocks_per_minute)
                if ($eta) { $line += " body_eta=$eta" }
                $pct = [math]::Round(100.0 * [double]$info.dogego_contiguous_raw_height / [double]$info.headers, 2)
                $line += " body_pct=$pct"
            }
        }
        if ($info.PSObject.Properties.Name -contains "dogego_body_ibd_header_paused" -and $info.dogego_body_ibd_header_paused -eq $true) {
            $line += " hdr_paused=1"
            $bodySnap = Get-DogeGoBodyIBDSnapshot $info
            if ($null -ne $bodySnap.header_resume_blocks) {
                $line += " hdr_resume=$($bodySnap.header_resume_blocks)"
                if ($bodySnap.header_resume_eta_text) { $line += " hdr_resume_eta=$($bodySnap.header_resume_eta_text)" }
            }
        }
        if ($info.PSObject.Properties.Name -contains "dogego_headers_sync_progress") {
            $line += (" hdr%={0:N1}" -f ([double]$info.dogego_headers_sync_progress * 100))
        }
        Write-Host $line
        if ($info.PSObject.Properties.Name -contains "dogego_post_aux_era_header_stall" -and $info.dogego_post_aux_era_header_stall -eq $true) {
            Write-Host "  post-aux era header stall risk (~510k) - rebuild dogego.exe if aux chain-id errors persist" -ForegroundColor Yellow
        }
        if ($info.PSObject.Properties.Name -contains "dogego_header_sync_recovery" -and $info.dogego_header_sync_recovery) {
            Write-Host "  recovery: $($info.dogego_header_sync_recovery)" -ForegroundColor Yellow
        }
        if ($info.initialblockdownload -eq $true -and $info.headers -ge 500000 -and $info.headers -lt 520000) {
            Write-Host "  NOTE: headers near ~510k (post-aux era). If headers stop climbing, rebuild dogego.exe - see docs/CORE_OPERATOR_RUNBOOK.md" -ForegroundColor Yellow
        }
        if ($info.headers -ge 510000 -and $script:LastHeaders -ne $null -and $info.headers -eq $script:LastHeaders -and $info.initialblockdownload -eq $true) {
            Write-Host "  WARNING: headers unchanged since last poll - check logs for auxpow/header errors" -ForegroundColor Red
        }
        $script:LastHeaders = $info.headers
    }
    $sum = Get-Summary
    if ($sum -and $sum.upnp_mapped) {
        Write-Host "  UPnP mapped: $($sum.upnp_external) ($($sum.upnp_method))" -ForegroundColor Green
    }
    if ($sum -and $sum.dogego_sync_status) {
        Write-Host "  $($sum.dogego_sync_status)" -ForegroundColor DarkGray
    }
    if ($sum -and $sum.rpc_status_display) {
        Write-Host "  rpc: $($sum.rpc_status_display)" -ForegroundColor DarkGray
    }
    Start-Sleep -Seconds $IntervalSec
}
