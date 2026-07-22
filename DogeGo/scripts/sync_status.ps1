# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# One-shot sync snapshot: on-disk checkpoints plus optional live RPC.
# Usage:
#   .\scripts\sync_status.ps1
#   .\scripts\sync_status.ps1 -Json
#   .\scripts\sync_status.ps1 -DataDir .\dogedata
param(
    [string]$DataDir,
    [string]$Network = "mainnet",
    [switch]$Json
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"
$rpcUp = $false
$rpcInfo = $null
$webSummary = $null
$rpcErr = $null
$conf = Read-DogeGoConfig
$root = Get-DogeGoRepoRoot
if (-not $DataDir) {
    $conf = Read-DogeGoConfig
    if ($conf.datadir) { $DataDir = $conf.datadir }
    else { $DataDir = Join-Path $root "dogedata" }
}
if (-not [System.IO.Path]::IsPathRooted($DataDir)) {
    $DataDir = Join-Path $root $DataDir
}
$chainDir = Get-DogeGoChainDir -DataDir $DataDir -Network $Network
$disk = Get-DogeGoDiskSyncSnapshot -DataDir $DataDir -Network $Network
$hasProcessLock = Test-Path (Join-Path $chainDir ".dogego-process.lock")

try {
    $rpcInfo = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 2 -WarmupDelaySec 2
    $rpcUp = $true
} catch {
    $rpcInfo = $null
    $rpcErr = $_.Exception.Message
    if ($rpcErr -match "warming up|-28") {
        try {
            $webSummary = Get-DogeGoWebSummary
        } catch {
            $webSummary = $null
        }
    } elseif (-not (Test-DogeGoRpcConfigured)) {
        try {
            $webSummary = Get-DogeGoWebSummary
        } catch {
            $webSummary = $null
        }
    }
}

if ($Json) {
    $out = [ordered]@{
        chain_dir  = $chainDir
        header_tip = $disk.HeaderTip
        raw_probe  = $disk.RawProbe
        body_pct   = $disk.BodyPct
        lag        = $disk.Lag
        process_lock = $hasProcessLock
        rpc_up     = $rpcUp
    }
    if ($rpcUp -and $rpcInfo) {
        $out.headers = [int64]$rpcInfo.headers
        $out.blocks = [int64]$rpcInfo.blocks
        $out.ibd = [bool]$rpcInfo.initialblockdownload
        if ($rpcInfo.PSObject.Properties.Name -contains "dogego_contiguous_raw_height") {
            $out.contiguous_raw = [int64]$rpcInfo.dogego_contiguous_raw_height
        }
        $cl = Get-DogeGoRpcConnectLag $rpcInfo
        if ($null -ne $cl) { $out.connect_lag = $cl }
        $cRate = Get-DogeGoRpcConnectBlocksPerMinute $rpcInfo
        if ($null -ne $cRate) { $out.connect_blocks_per_minute = $cRate }
        $boost = Format-DogeGoConnectCatchUpBoost (Get-DogeGoRpcConnectCatchUpTuning $rpcInfo)
        if ($boost) { $out.connect_catch_up_boost = $boost }
        if ($rpcInfo.PSObject.Properties.Name -contains "dogego_raw_sync" -and $rpcInfo.dogego_raw_sync.blocks_per_minute) {
            $out.blocks_per_minute = [double]$rpcInfo.dogego_raw_sync.blocks_per_minute
        }
        if ($rpcInfo.PSObject.Properties.Name -contains "dogego_body_ibd_header_paused") {
            $out.body_ibd_header_paused = [bool]$rpcInfo.dogego_body_ibd_header_paused
        }
        if ($rpcInfo.PSObject.Properties.Name -contains "dogego_sync_health") {
            $out.sync_health = [string]$rpcInfo.dogego_sync_health
        }
    } elseif ($webSummary) {
        $out.webui = $true
        $out.headers = [int64]$webSummary.tip_height
        $out.blocks = [int64]$webSummary.chain_active_height
        $out.contiguous_raw = [int64]$webSummary.contiguous_raw_height
        $out.body_ibd_header_paused = [bool]$webSummary.dogego_body_ibd_header_paused
        $out.sync_health = [string]$webSummary.dogego_sync_health
        $webBoost = [ordered]@{
            passes      = if ($webSummary.dogego_connect_catch_up_passes) { [int64]$webSummary.dogego_connect_catch_up_passes } else { $null }
            batch       = if ($webSummary.dogego_connect_catch_up_batch) { [int64]$webSummary.dogego_connect_catch_up_batch } else { $null }
            interval_ms = if ($webSummary.dogego_connect_catch_up_interval_ms) { [int64]$webSummary.dogego_connect_catch_up_interval_ms } else { $null }
        }
        $boost = Format-DogeGoConnectCatchUpBoost $webBoost
        if ($boost) { $out.connect_catch_up_boost = $boost }
    }
    $out | ConvertTo-Json -Compress
    exit 0
}

function Format-CheckpointAge($mtime) {
    if (-not $mtime) { return "" }
    $age = (Get-Date) - $mtime
    if ($age.TotalHours -ge 1) {
        return (" (checkpoint {0:N1}h ago)" -f $age.TotalHours)
    }
    if ($age.TotalMinutes -ge 1) {
        return (" (checkpoint {0:N0}m ago)" -f $age.TotalMinutes)
    }
    return " (checkpoint just now)"
}

Write-Host "=== DogeGo sync status ===" -ForegroundColor Cyan
Write-Host ("chain dir: {0}" -f $chainDir)
if ($null -ne $disk.HeaderTip) {
    Write-Host ("disk headers tip: {0} (layout={1}){2}" -f $disk.HeaderTip, $disk.HeaderLayout, (Format-CheckpointAge $disk.HeaderSyncMtime))
} else {
    Write-Host "disk headers: (no headers_sync.json)" -ForegroundColor DarkGray
}
if ($null -ne $disk.RawProbe) {
    Write-Host ("disk raw sync probe: {0}{1}" -f $disk.RawProbe, (Format-CheckpointAge $disk.RawSyncMtime))
} else {
    Write-Host "disk rawblocks: (no rawblocks_sync.json)" -ForegroundColor DarkGray
}
if ($null -ne $disk.BodyPct) {
    $pctStr = $disk.BodyPct.ToString("F3", [System.Globalization.CultureInfo]::InvariantCulture)
    Write-Host ("disk body coverage (probe/header): {0}%" -f $pctStr) -ForegroundColor DarkGray
}

if ($rpcUp -and $rpcInfo) {
    Write-Host ("RPC headers={0} blocks={1} ibd={2}" -f $rpcInfo.headers, $rpcInfo.blocks, $rpcInfo.initialblockdownload)
    if ($rpcInfo.PSObject.Properties.Name -contains "dogego_contiguous_raw_height") {
        Write-Host ("RPC contiguous_raw={0}" -f $rpcInfo.dogego_contiguous_raw_height)
        if ($rpcInfo.PSObject.Properties.Name -contains "dogego_raw_sync") {
            $rsGap = $rpcInfo.dogego_raw_sync
            if ($null -ne $rsGap.lowest_missing_height -and $rsGap.lowest_missing_height -gt ([int64]$rpcInfo.dogego_contiguous_raw_height + 1)) {
                $hole = $rsGap.lowest_missing_height - [int64]$rpcInfo.dogego_contiguous_raw_height - 1
                Write-Host ("raw body gap: {0} heights stored ahead of contiguous (connect uses contiguous only)" -f $hole) -ForegroundColor Yellow
            }
        }
    }
    $rpcConnectLag = Get-DogeGoRpcConnectLag $rpcInfo
    $rpcConnectRate = Get-DogeGoRpcConnectBlocksPerMinute $rpcInfo
    if ($null -ne $rpcConnectLag -and $rpcConnectLag -gt 0) {
        Write-Host ("RPC connect_lag={0} (stored bodies ahead of chainActive)" -f $rpcConnectLag) -ForegroundColor DarkGray
    }
    if ($null -ne $rpcConnectRate) {
        Write-Host ("connect rate={0:N1} blocks/min" -f $rpcConnectRate) -ForegroundColor DarkGray
        if ($null -ne $rpcConnectLag) {
            $lag = [int64]$rpcConnectLag
            $rate = [double]$rpcConnectRate
            if ($lag -gt 0 -and $rate -gt 0) {
                $etaMin = [math]::Ceiling($lag / $rate)
                Write-Host ("connect catch-up ETA: ~{0} min" -f $etaMin) -ForegroundColor DarkGray
            }
        }
    }
    $boostLine = Format-DogeGoConnectCatchUpBoost (Get-DogeGoRpcConnectCatchUpTuning $rpcInfo)
    if ($boostLine) {
        Write-Host ("connect boost={0}" -f $boostLine) -ForegroundColor DarkGray
    }
    if ($rpcInfo.PSObject.Properties.Name -contains "dogego_raw_sync") {
        $rs = $rpcInfo.dogego_raw_sync
        if ($rs.lowest_missing_height -ge 0) {
            Write-Host ("body frontier: missing from height {0}" -f $rs.lowest_missing_height) -ForegroundColor DarkGray
        }
        if ($null -ne $rs.in_flight_batches) {
            Write-Host ("body download: in_flight={0} workers={1}" -f $rs.in_flight_batches, $rs.sync_workers) -ForegroundColor DarkGray
        }
        if ($rs.last_block_stall_peer) {
            Write-Host ("last block stall peer: {0}" -f $rs.last_block_stall_peer) -ForegroundColor DarkGray
        }
        if ($rs.last_block_stored_at) {
            $ageMin = [math]::Round(((Get-Date) - [DateTimeOffset]::FromUnixTimeSeconds([int64]$rs.last_block_stored_at).LocalDateTime).TotalMinutes, 1)
            Write-Host ("last body stored: {0} min ago" -f $ageMin) -ForegroundColor DarkGray
        }
		if ($rs.blocks_per_minute) {
            Write-Host ("body IBD rate={0:N1} blocks/min" -f [double]$rs.blocks_per_minute) -ForegroundColor DarkGray
            if ($null -ne $rpcInfo.headers -and $rpcInfo.PSObject.Properties.Name -contains "dogego_contiguous_raw_height") {
                $eta = Format-DogeGoBodyIBDEta -HeaderTip ([int64]$rpcInfo.headers) -Contiguous ([int64]$rpcInfo.dogego_contiguous_raw_height) -BlocksPerMinute ([double]$rs.blocks_per_minute)
                if ($eta) {
                    Write-Host ("body IBD ETA: {0}" -f $eta) -ForegroundColor DarkGray
                }
            }
        }
    } elseif ($rpcInfo.PSObject.Properties.Name -contains "dogego_raw_sync" -and $rpcInfo.dogego_raw_sync.blocks_per_minute) {
        Write-Host ("body IBD rate={0:N1} blocks/min" -f [double]$rpcInfo.dogego_raw_sync.blocks_per_minute) -ForegroundColor DarkGray
    }
    if ($rpcInfo.PSObject.Properties.Name -contains "dogego_sync_health" -and $rpcInfo.dogego_sync_health) {
        Write-Host ("sync_health={0}" -f $rpcInfo.dogego_sync_health) -ForegroundColor DarkGray
    }
    if ($rpcInfo.PSObject.Properties.Name -contains "dogego_auxpow_parent_chain_id_core_parity") {
        Write-Host ("auxpow_core_parity={0}" -f $rpcInfo.dogego_auxpow_parent_chain_id_core_parity)
    }
    if ($rpcInfo.PSObject.Properties.Name -contains "dogego_body_ibd_header_paused" -and $rpcInfo.dogego_body_ibd_header_paused) {
        Write-Host "body_ibd_header_paused=true" -ForegroundColor DarkGray
        $bodySnap = Get-DogeGoBodyIBDSnapshot $rpcInfo
        if ($null -ne $bodySnap.header_resume_blocks) {
            $hrLine = "header resume at contiguous ~$($bodySnap.header_resume_contiguous) ($($bodySnap.header_resume_blocks) blocks remaining)"
            if ($bodySnap.header_resume_eta_text) { $hrLine += " ETA $($bodySnap.header_resume_eta_text)" }
            Write-Host $hrLine -ForegroundColor DarkGray
        }
    }
    if ($rpcInfo.PSObject.Properties.Name -contains "dogego_post_aux_era_header_stall" -and $rpcInfo.dogego_post_aux_era_header_stall) {
        Write-Host "post_aux_era_header_stall=true" -ForegroundColor Yellow
    }
} else {
    if ($rpcErr -match "warming up|-28") {
        Write-Host "RPC: warming up (port open; chain init in progress)" -ForegroundColor Yellow
    } elseif (-not (Test-DogeGoRpcConfigured)) {
        Write-Host "RPC: using default 127.0.0.1:22557 (set `"rpc`" in dogecoinconf.json to override)" -ForegroundColor DarkGray
    } else {
        Write-Host "RPC: unavailable (try .\scripts\resume_node.ps1)" -ForegroundColor DarkGray
    }
    if ($webSummary) {
        $rpcLine = ""
        if ($webSummary.rpc_status_display) { $rpcLine = " rpc=$($webSummary.rpc_status_display)" }
        Write-Host ("Web UI: headers={0} contiguous_raw={1} sync_health={2}{3}" -f $webSummary.tip_height, $webSummary.contiguous_raw_height, $webSummary.dogego_sync_health, $rpcLine)
        if ($webSummary.dogego_body_ibd_header_paused -eq $true) {
            Write-Host "body_ibd_header_paused=true" -ForegroundColor DarkGray
        }
        $webBoost = [ordered]@{
            passes      = if ($webSummary.dogego_connect_catch_up_passes) { [int64]$webSummary.dogego_connect_catch_up_passes } else { $null }
            batch       = if ($webSummary.dogego_connect_catch_up_batch) { [int64]$webSummary.dogego_connect_catch_up_batch } else { $null }
            interval_ms = if ($webSummary.dogego_connect_catch_up_interval_ms) { [int64]$webSummary.dogego_connect_catch_up_interval_ms } else { $null }
        }
        $boostLine = Format-DogeGoConnectCatchUpBoost $webBoost
        if ($boostLine) {
            Write-Host ("connect boost={0}" -f $boostLine) -ForegroundColor DarkGray
        }
    }
    $hdrSync = Join-Path $chainDir "headers_sync.json"
    $rawSync = Join-Path $chainDir "rawblocks_sync.json"
    if ((Test-Path $hdrSync) -and ((Get-Date) - (Get-Item $hdrSync).LastWriteTime).TotalHours -gt 2) {
        Write-Host "NOTE: headers checkpoint is stale - node may be stopped" -ForegroundColor Yellow
    }
    if ((Test-Path $rawSync) -and ((Get-Date) - (Get-Item $rawSync).LastWriteTime).TotalHours -gt 2) {
        Write-Host "NOTE: rawblocks checkpoint is stale - body IBD pauses when the node is off" -ForegroundColor Yellow
    }
}

if ($null -ne $disk.Lag) {
    if ($disk.Lag -gt 1000) {
        Write-Host ("header/body lag (disk probe): ~{0} heights - normal during mainnet IBD" -f $disk.Lag) -ForegroundColor DarkGray
    }
    if ($disk.Lag -gt 50000) {
        Write-Host "header sync: paused while bodies catch up (DogeGo defers getheaders when the peer is near your local tip)" -ForegroundColor DarkGray
        Write-Host "watch progress: rawblocks_sync.json next_probe_height (not header %)" -ForegroundColor DarkGray
    }
}
if ($hasProcessLock) {
    Write-Host "process_lock=active" -ForegroundColor DarkGray
} elseif (Get-Process dogego -ErrorAction SilentlyContinue) {
    Write-Host 'WARN: dogego running but process lock missing - restart with .\scripts\restart_node.ps1 -Rebuild' -ForegroundColor Yellow
}
