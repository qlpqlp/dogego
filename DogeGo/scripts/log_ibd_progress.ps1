# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Append mainnet IBD progress to a CSV log (for long soak / certification).
# Uses JSON-RPC when the node is up; falls back to disk checkpoints when RPC is down.
#
# Usage:
#   .\scripts\log_ibd_progress.ps1 -OutFile ibd_progress.csv -IntervalSec 60
#   .\scripts\log_ibd_progress.ps1 -DiskOnly -IntervalSec 120
param(
    [string]$OutFile = "ibd_progress.csv",
    [int]$IntervalSec = 60,
    [switch]$DiskOnly,
    [string]$DataDir,
    [string]$Network = "mainnet",
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
    TimeoutSec     = 120
}
if ($RpcUser) { $rpcParams.RpcUser = $RpcUser }
if ($RpcPassword) { $rpcParams.RpcPassword = $RpcPassword }
if ($RpcHost) { $rpcParams.RpcHost = $RpcHost }
if ($RpcPort -gt 0) { $rpcParams.RpcPort = $RpcPort }
$path = Join-Path (Get-Location) $OutFile
if (-not (Test-Path $path)) {
    "timestamp,source,headers,blocks,ibd,contiguous_raw,raw_probe,connect_lag,body_pct,body_eta_min,download_per_min,connect_per_min,hdr_progress,post_aux_stall,parity_flag,warnings,connect_boost" | Out-File -FilePath $path -Encoding utf8
}
Write-Host "Logging to $path every ${IntervalSec}s (Ctrl+C to stop)" -ForegroundColor Cyan
if ($DiskOnly) { Write-Host "Mode: disk-only (rawblocks_sync.json + headers_sync.json)" -ForegroundColor DarkGray }
$lastProbe = $null
while ($true) {
    $ts = (Get-Date).ToString("o")
    $source = "disk"
    $headers = ""
    $blocks = ""
    $ibd = ""
    $cont = ""
    $probe = ""
    $connectLag = ""
    $bodyPct = ""
    $bodyEtaMin = ""
    $downloadPerMin = ""
    $connectPerMin = ""
    $hdrPct = ""
    $stall = ""
    $parity = ""
    $warn = ""
    $connectBoost = ""
    if (-not $DiskOnly) {
        try {
            $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo @rpcParams
            $source = "rpc"
            $headers = $info.headers
            $blocks = $info.blocks
            $ibd = $info.initialblockdownload
            if ($info.PSObject.Properties.Name -contains "dogego_contiguous_raw_height") {
                $cont = $info.dogego_contiguous_raw_height
            }
            $cl = Get-DogeGoRpcConnectLag $info
            if ($null -ne $cl) { $connectLag = $cl }
            if ($info.PSObject.Properties.Name -contains "dogego_raw_sync") {
                $rs = $info.dogego_raw_sync
                if ($null -ne $rs -and $rs.blocks_per_minute) {
                    $downloadPerMin = [math]::Round([double]$rs.blocks_per_minute, 2)
                    if ($info.PSObject.Properties.Name -contains "dogego_contiguous_raw_height") {
                        $eta = Get-DogeGoBodyIBDEtaMinutes -HeaderTip ([int64]$info.headers) -Contiguous ([int64]$info.dogego_contiguous_raw_height) -BlocksPerMinute ([double]$rs.blocks_per_minute)
                        if ($null -ne $eta) { $bodyEtaMin = $eta }
                    }
                }
            }
            $cRate = Get-DogeGoRpcConnectBlocksPerMinute $info
            if ($null -ne $cRate) {
                $connectPerMin = [math]::Round($cRate, 2)
            }
            $boost = Format-DogeGoConnectCatchUpBoost (Get-DogeGoRpcConnectCatchUpTuning $info)
            if ($boost) { $connectBoost = $boost }
            if ($info.PSObject.Properties.Name -contains "dogego_headers_sync_progress") {
                $hdrPct = [math]::Round([double]$info.dogego_headers_sync_progress * 100, 2)
            }
            $stall = $info.dogego_post_aux_era_header_stall
            $parity = $info.dogego_auxpow_parent_chain_id_core_parity
            if ($null -ne $info.warnings) {
                $warn = ([string]$info.warnings -replace '"', '""')
            }
        } catch {
            $err = if ($_.Exception.Message) { $_.Exception.Message } else { "$_" }
            Write-Host "[$ts] RPC unavailable ($err), using disk checkpoints" -ForegroundColor DarkGray
        }
    }
    if ($source -eq "disk") {
        $disk = Get-DogeGoDiskSyncSnapshot -DataDir $DataDir -Network $Network
        if ($null -ne $disk.HeaderTip) { $headers = $disk.HeaderTip }
        if ($null -ne $disk.RawProbe) {
            $probe = $disk.RawProbe
            $blocks = [math]::Max(0, $disk.RawProbe - 1)
        }
        if ($null -ne $disk.BodyPct) { $bodyPct = $disk.BodyPct }
    }
    $rateHint = ""
    if ($null -ne $lastProbe -and $probe -ne "" -and [int64]$probe -gt $lastProbe) {
        $delta = [int64]$probe - $lastProbe
        $rateHint = (" +{0} heights/{1}s" -f $delta, $IntervalSec)
        $lastProbe = [int64]$probe
    } elseif ($probe -ne "" -and $null -eq $lastProbe) {
        $lastProbe = [int64]$probe
    }
    $line = "{0},{1},{2},{3},{4},{5},{6},{7},{8},{9},{10},{11},{12},{13},{14},""{15}"",""{16}""" -f $ts, $source, $headers, $blocks, $ibd, $cont, $probe, $connectLag, $bodyPct, $bodyEtaMin, $downloadPerMin, $connectPerMin, $hdrPct, $stall, $parity, $warn, ($connectBoost -replace '"', '""')
    Add-Content -Path $path -Value $line -Encoding utf8
    Write-Host ($line + $rateHint)
    Start-Sleep -Seconds $IntervalSec
}
