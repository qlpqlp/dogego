# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Prove mainnet IBD is making forward progress (body download or chain connect).
# Uses JSON-RPC when available; falls back to on-disk rawblocks_sync.json probe height.
#
#   .\scripts\ibd_convergence_check.ps1
#   .\scripts\ibd_convergence_check.ps1 -IntervalSec 180 -MinRawProbeAdvance 50
#   .\scripts\ibd_convergence_check.ps1 -DiskOnly
param(
    [int]$IntervalSec = 120,
    [int64]$MinContiguousAdvance = 1,
    [int64]$MinBlocksAdvance = 1,
    [int64]$MinRawProbeAdvance = 1,
    [int64]$MaxContiguousRegression = 64,
    [string]$DataDir,
    [string]$Network = "mainnet",
    [string]$RpcUser,
    [string]$RpcPassword,
    [string]$RpcHost,
    [int]$RpcPort = 0,
    [switch]$DiskOnly
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

$rpcParams = @{
    WarmupRetries  = 8
    WarmupDelaySec = 3
}
if ($RpcUser) { $rpcParams.RpcUser = $RpcUser }
if ($RpcPassword) { $rpcParams.RpcPassword = $RpcPassword }
if ($RpcHost) { $rpcParams.RpcHost = $RpcHost }
if ($RpcPort -gt 0) { $rpcParams.RpcPort = $RpcPort }

function Get-ProgressSnapshot {
    param([switch]$UseDiskOnly)
    $snap = [ordered]@{
        Source      = "none"
        Headers     = $null
        Blocks      = $null
        Contiguous  = $null
        RawProbe    = $null
        ReplayTarget = $null
        IBD         = $null
        RpcReady    = $false
        ConnectBoost = $null
    }
    if (-not $UseDiskOnly) {
        $sync = Get-DogeGoSyncHeightsSnapshot -RpcTimeoutSec 45 -RpcWarmupRetries 8
        if ($sync.source) {
            $snap.Source = [string]$sync.source
            $snap.RpcReady = ($sync.source -match "rpc")
        }
        if ($null -ne $sync.blocks) { $snap.Blocks = [int64]$sync.blocks }
        if ($null -ne $sync.stored) { $snap.Contiguous = [int64]$sync.stored }
        try {
            $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 1 -WarmupDelaySec 2 -TimeoutSec 45
            $snap.Headers = [int64]$info.headers
            $snap.IBD = [bool]$info.initialblockdownload
            if ($info.PSObject.Properties.Name -contains "dogego_utxo_replay_target") {
                $snap.ReplayTarget = [int64]$info.dogego_utxo_replay_target
            } elseif ($info.PSObject.Properties.Name -contains "dogego_utxo_chain_active") {
                $snap.ReplayTarget = [int64]$info.dogego_utxo_chain_active
            }
            if ($null -eq $snap.Blocks) { $snap.Blocks = [int64]$info.blocks }
            if ($null -eq $snap.Contiguous -and $info.PSObject.Properties.Name -contains "dogego_contiguous_raw_height") {
                $snap.Contiguous = [int64]$info.dogego_contiguous_raw_height
            }
            $boost = Format-DogeGoConnectCatchUpBoost (Get-DogeGoRpcConnectCatchUpTuning $info)
            if ($boost) { $snap.ConnectBoost = $boost }
            $snap.RpcReady = $true
            if ($snap.Source -eq "none") { $snap.Source = "rpc" }
        } catch {
            if ($_.Exception.Message -notmatch "warming up|-28") {
                Write-Host ("RPC detail failed: {0}" -f $_.Exception.Message) -ForegroundColor DarkGray
            }
        }
    }
    if ($null -eq $snap.Headers) {
        try {
            $web = Get-DogeGoWebSummary
            if ($null -ne $web.tip_height) { $snap.Headers = [int64]$web.tip_height }
            if ($null -eq $snap.Blocks -and $null -ne $web.chain_active_height) { $snap.Blocks = [int64]$web.chain_active_height }
            if ($null -eq $snap.Contiguous -and $null -ne $web.contiguous_raw_height) { $snap.Contiguous = [int64]$web.contiguous_raw_height }
            if ($null -eq $snap.ConnectBoost) {
                $webBoost = [ordered]@{
                    passes      = if ($web.dogego_connect_catch_up_passes) { [int64]$web.dogego_connect_catch_up_passes } else { $null }
                    batch       = if ($web.dogego_connect_catch_up_batch) { [int64]$web.dogego_connect_catch_up_batch } else { $null }
                    interval_ms = if ($web.dogego_connect_catch_up_interval_ms) { [int64]$web.dogego_connect_catch_up_interval_ms } else { $null }
                }
                $boost = Format-DogeGoConnectCatchUpBoost $webBoost
                if ($boost) { $snap.ConnectBoost = $boost }
            }
            if ($snap.Source -eq "none") { $snap.Source = "webui" }
        } catch { }
    }
    if ($snap.Source -eq "none" -or $null -eq $snap.RawProbe) {
        $disk = Get-DogeGoDiskSyncSnapshot -DataDir $DataDir -Network $Network
        if ($null -ne $disk.HeaderTip) { $snap.Headers = [int64]$disk.HeaderTip }
        if ($null -ne $disk.RawProbe) {
            $snap.RawProbe = [int64]$disk.RawProbe
            if ($snap.Source -eq "none") { $snap.Source = "disk" }
        }
    }
    return [pscustomobject]$snap
}

function Format-Snap($s) {
    $parts = @("source=$($s.Source)")
    if ($null -ne $s.Headers) { $parts += "headers=$($s.Headers)" }
    if ($null -ne $s.Blocks) { $parts += "blocks=$($s.Blocks)" }
    if ($null -ne $s.Contiguous) { $parts += "contiguous=$($s.Contiguous)" }
    if ($null -ne $s.ReplayTarget) { $parts += "replay_target=$($s.ReplayTarget)" }
    if ($null -ne $s.RawProbe) { $parts += "raw_probe=$($s.RawProbe)" }
    if ($null -ne $s.IBD) { $parts += "ibd=$($s.IBD)" }
    if ($s.ConnectBoost) { $parts += "connect_boost=$($s.ConnectBoost)" }
    return ($parts -join " ")
}

function Get-RawSyncInFlight($Info) {
    if ($null -eq $Info) { return $null }
    if ($Info.PSObject.Properties.Name -contains "dogego_raw_sync") {
        $rs = $Info.dogego_raw_sync
        if ($rs -and $rs.PSObject.Properties.Name -contains "in_flight_batches") {
            return [int64]$rs.in_flight_batches
        }
    }
    return $null
}

function Test-ConnectCaughtUp($Snap) {
    if ($null -eq $Snap.Blocks -or $null -eq $Snap.Contiguous) { return $false }
    return ($Snap.Blocks -eq $Snap.Contiguous) -and ($Snap.Contiguous -ge 0)
}

Write-Host "=== DogeGo IBD convergence check ===" -ForegroundColor Cyan
Write-Host ("Interval: {0}s  thresholds: contiguous+>={1} blocks+>={2} raw_probe+>={3}" -f $IntervalSec, $MinContiguousAdvance, $MinBlocksAdvance, $MinRawProbeAdvance)

$a = Get-ProgressSnapshot -UseDiskOnly:$DiskOnly
if ($a.Source -eq "none") {
    Write-Host "FAIL: no RPC and no disk checkpoints (is datadir correct?)" -ForegroundColor Red
    exit 2
}
Write-Host ("T0: {0}" -f (Format-Snap $a))

Start-Sleep -Seconds $IntervalSec

$b = Get-ProgressSnapshot -UseDiskOnly:$DiskOnly
if ($b.Source -eq "none") {
    Write-Host "FAIL: lost progress visibility at T1" -ForegroundColor Red
    exit 2
}
Write-Host ("T1: {0}" -f (Format-Snap $b))

$contAdv = 0
$blockAdv = 0
$probeAdv = 0
if ($null -ne $a.Contiguous -and $null -ne $b.Contiguous) {
    $contAdv = $b.Contiguous - $a.Contiguous
}
if ($null -ne $a.Blocks -and $null -ne $b.Blocks) {
    $blockAdv = $b.Blocks - $a.Blocks
}
if ($null -ne $a.RawProbe -and $null -ne $b.RawProbe) {
    $probeAdv = $b.RawProbe - $a.RawProbe
}

Write-Host ("Advance: contiguous=+{0} blocks=+{1} raw_probe=+{2}" -f $contAdv, $blockAdv, $probeAdv)

if ($contAdv -lt -$MaxContiguousRegression) {
    Write-Host ("FAIL: contiguous regression {0} (drop > {1}) - header rewind or cache wipe suspected" -f $contAdv, $MaxContiguousRegression) -ForegroundColor Red
    exit 3
}

if ($null -ne $b.ReplayTarget -and $null -ne $b.Contiguous -and $b.ReplayTarget -gt ($b.Contiguous + 1)) {
    $remain = $b.ReplayTarget - $b.Contiguous
    $pct = [math]::Round(100.0 * $b.Contiguous / $b.ReplayTarget, 1)
    Write-Host ("Snapshot body replay: {0}/{1} ({2}%; ~{3} left)" -f $b.Contiguous, $b.ReplayTarget, $pct, $remain) -ForegroundColor DarkGray
}

if ($null -ne $a.Blocks -and $null -ne $b.Blocks -and $IntervalSec -gt 0 -and $blockAdv -gt 0) {
    $connectRate = [math]::Round($blockAdv * 60.0 / $IntervalSec, 1)
    Write-Host ("Implied connect rate: ~{0} blocks/min" -f $connectRate) -ForegroundColor DarkGray
}
if ($null -ne $a.Contiguous -and $null -ne $b.Contiguous -and $null -ne $a.Blocks -and $null -ne $b.Blocks) {
    $lag0 = $a.Contiguous - $a.Blocks
    $lag1 = $b.Contiguous - $b.Blocks
    if ($lag0 -gt 0 -or $lag1 -gt 0) {
        Write-Host ("Connect lag: {0} -> {1} (delta {2})" -f $lag0, $lag1, ($lag1 - $lag0)) -ForegroundColor DarkGray
    }
}

$ok = $false
if ($contAdv -ge $MinContiguousAdvance) { $ok = $true }
if ($blockAdv -ge $MinBlocksAdvance) { $ok = $true }
if ($probeAdv -ge $MinRawProbeAdvance) { $ok = $true }

$connectCaughtUp = (Test-ConnectCaughtUp $a) -and (Test-ConnectCaughtUp $b)
if (-not $ok -and $connectCaughtUp) {
    $inFlight = Get-RawSyncInFlight $b
    if ($null -ne $inFlight -and $inFlight -gt 0) {
        Write-Host ("Body-only IBD: connect caught up (blocks=contiguous={0}); in_flight_batches={1}" -f $b.Contiguous, $inFlight) -ForegroundColor DarkGray
        $ok = $true
    }
}

if (-not $ok) {
    Write-Host "FAIL: no measurable IBD progress in window (node stopped or stalled)" -ForegroundColor Red
    try {
        $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 1 -WarmupDelaySec 2 -TimeoutSec 30
        if ($info.dogego_raw_sync) {
            $rs = $info.dogego_raw_sync
            $hints = @()
            $lastBodyMins = $null
            if ($rs.last_block_stored_at) {
                $lastBodyMins = [math]::Round(((Get-Date) - [DateTimeOffset]::FromUnixTimeSeconds([int64]$rs.last_block_stored_at).LocalDateTime).TotalMinutes, 1)
                $hints += "last_body_store=${lastBodyMins}m_ago"
            }
            if ($null -ne $rs.in_flight_batches) { $hints += "in_flight=$($rs.in_flight_batches)" }
            if ($null -ne $rs.assist_active_sessions) { $hints += "assist_sessions=$($rs.assist_active_sessions)" }
            if ($null -ne $rs.assist_peer_pool) { $hints += "assist_pool=$($rs.assist_peer_pool)" }
            if ($hints.Count -gt 0) {
                Write-Host ("Hint: {0}" -f ($hints -join " ")) -ForegroundColor DarkGray
            }
            if ($null -ne $lastBodyMins -and $lastBodyMins -gt 5 -and $rs.in_flight_batches -eq 0) {
                Write-Host 'Rebuild dogego.exe and restart - body IBD pump requires current binary (ibd_body_pump.go)' -ForegroundColor DarkGray
            }
        }
    } catch { }
    Write-Host "Check Web UI http://127.0.0.1:2013 or .\scripts\sync_status.ps1" -ForegroundColor DarkGray
    exit 1
}

Write-Host "OK: IBD forward progress confirmed." -ForegroundColor Green
if ($null -ne $b.Headers -and $null -ne $b.Contiguous -and $b.Headers -gt 0) {
    $pct = [math]::Round(100.0 * $b.Contiguous / $b.Headers, 2)
    Write-Host ('Body coverage: {0}/{1} ({2}%)' -f $b.Contiguous, $b.Headers, $pct) -ForegroundColor DarkGray
}
exit 0
