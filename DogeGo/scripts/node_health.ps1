# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# One-shot mainnet full-node health check (Core operator parity).
# Usage:
#   .\scripts\node_health.ps1
#   .\scripts\node_health.ps1 -Json
param(
    [switch]$Json,
    [string]$DataDir,
    [string]$Network = "mainnet",
    [int]$RpcPort = 0
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

$rpcParams = @{ WarmupRetries = 3; WarmupDelaySec = 2 }
if ($RpcPort -gt 0) { $rpcParams.RpcPort = $RpcPort }

$disk = Get-DogeGoDiskSyncSnapshot -DataDir $DataDir -Network $Network
$lockPath = Join-Path $disk.ChainDir ".dogego-process.lock"
$hasProcessLock = Test-Path $lockPath
$rpcUp = $false
$rpcReady = $false
$info = $null
$web = $null
$issues = @()
$warnings = @()
$notes = @()

try {
    $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo @rpcParams
    $rpcUp = $true
    $rpcReady = $true
} catch {
    if ($_.Exception.Message -match "warming up|-28") {
        $rpcUp = $true
        $warnings += "rpc_warming_up"
    } elseif ($_.Exception.Message -match "timed out|timeout|TimeoutSec|operation has timed out") {
        $warnings += "rpc_busy_timeout"
        if ($hasProcessLock) { $rpcUp = $true }
    } elseif ($_.Exception.Message -match "Unable to connect|failed:") {
        $issues += "rpc_unreachable"
    } else {
        $issues += ("rpc_error: " + $_.Exception.Message)
    }
}

try {
    $web = Get-DogeGoWebSummary
} catch {
    $warnings += "webui_unreachable"
}

if (-not $rpcUp -and $web -and ($web.rpc_listening -eq $true -or $web.rpc_enabled -eq $true)) {
    try {
        $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo @rpcParams
        $rpcUp = $true
        $rpcReady = $true
        $issues = @($issues | Where-Object { $_ -ne "rpc_unreachable" })
    } catch {
        if ($_.Exception.Message -match "warming up|-28") {
            $rpcUp = $true
            $warnings += "rpc_warming_up"
            $issues = @($issues | Where-Object { $_ -ne "rpc_unreachable" })
        } elseif ($_.Exception.Message -match "timed out|timeout|TimeoutSec|operation has timed out") {
            $warnings += "rpc_busy_timeout"
            $rpcUp = $true
            $issues = @($issues | Where-Object { $_ -ne "rpc_unreachable" })
        }
    }
}

# Web UI exposes in-process RPC dispatch when JSON-RPC port is disabled or still warming.
if ($web -and $web.rpc_dispatch_ready -eq $true) { $rpcReady = $true }
if ($web -and $web.rpc_listening -eq $true) {
    $rpcUp = $true
    $issues = @($issues | Where-Object { $_ -ne "rpc_unreachable" })
}
if ($issues -contains "rpc_unreachable" -and $web -and $null -ne $web.chain_active_height -and -not $rpcUp) {
    $issues = @($issues | Where-Object { $_ -ne "rpc_unreachable" })
    if ($web.rpc_enabled -eq $false) {
        $warnings += "rpc_port_disabled_use_webui"
        $notes += "enable_rpc_in_dogecoinconf_for_cli"
    } else {
        $warnings += "rpc_port_unreachable_use_webui"
    }
}

$headers = if ($info) { [int64]$info.headers } elseif ($web) { [int64]$web.tip_height } else { $disk.HeaderTip }
if ($warnings -contains "rpc_busy_timeout" -and ($hasProcessLock -or ($web -and $web.rpc_listening -eq $true))) {
    $rpcUp = $true
    $issues = @($issues | Where-Object { $_ -notmatch "^rpc_error:" })
    if ($web -and $null -ne $web.chain_active_height) {
        $rpcReady = $true
    } elseif ($hasProcessLock -and $null -ne $headers -and $headers -ge 510000) {
        $rpcReady = $true
    }
    $notes += "rpc_timeout_expected_during_heavy_ibd"
}
$blocks = if ($info) { [int64]$info.blocks } elseif ($web) { [int64]$web.chain_active_height } else { $null }
$cont = if ($info -and $info.PSObject.Properties.Name -contains "dogego_contiguous_raw_height") { [int64]$info.dogego_contiguous_raw_height }
        elseif ($web) { [int64]$web.contiguous_raw_height } else { $null }
$connectLag = if ($info) { Get-DogeGoRpcConnectLag $info } else { $null }
$connectRate = if ($info) { Get-DogeGoRpcConnectBlocksPerMinute $info } else { $null }
$connectBoost = if ($info) { Get-DogeGoRpcConnectCatchUpTuning $info } else {
    [ordered]@{ passes = $null; batch = $null; interval_ms = $null }
}
if ($null -eq $connectBoost.passes -and $web) {
    $connectBoost = [ordered]@{
        passes      = if ($web.dogego_connect_catch_up_passes) { [int64]$web.dogego_connect_catch_up_passes } else { $null }
        batch       = if ($web.dogego_connect_catch_up_batch) { [int64]$web.dogego_connect_catch_up_batch } else { $null }
        interval_ms = if ($web.dogego_connect_catch_up_interval_ms) { [int64]$web.dogego_connect_catch_up_interval_ms } else { $null }
    }
}
$connectBoostLine = Format-DogeGoConnectCatchUpBoost $connectBoost
$utxoConnectInFlight = if ($info -and $info.PSObject.Properties.Name -contains "dogego_utxo_connect_in_flight") { [bool]$info.dogego_utxo_connect_in_flight } else { $null }
$syncUtxoInFlight = if ($info -and $info.PSObject.Properties.Name -contains "dogego_syncutxo_in_flight") { [bool]$info.dogego_syncutxo_in_flight } else { $null }
$utxoSnapshotInFlight = if ($info -and $info.PSObject.Properties.Name -contains "dogego_utxo_snapshot_save_in_flight") { [bool]$info.dogego_utxo_snapshot_save_in_flight } else { $null }
$assistPool = $null
$assistSessions = $null
if ($info -and $info.PSObject.Properties.Name -contains "dogego_raw_sync" -and $info.dogego_raw_sync) {
    $rs = $info.dogego_raw_sync
    if ($rs.PSObject.Properties.Name -contains "assist_peer_pool") { $assistPool = [int]$rs.assist_peer_pool }
    if ($rs.PSObject.Properties.Name -contains "assist_active_sessions") { $assistSessions = [int]$rs.assist_active_sessions }
}
$downloadRate = $null
	if ($info -and $info.PSObject.Properties.Name -contains "dogego_raw_sync" -and $info.dogego_raw_sync.blocks_per_minute) {
    $downloadRate = [double]$info.dogego_raw_sync.blocks_per_minute
}
$rawSync = $null
if ($info -and $info.PSObject.Properties.Name -contains "dogego_raw_sync") {
    $rawSync = $info.dogego_raw_sync
}
$lastBodyStoreMin = $null
if ($rawSync -and $rawSync.last_block_stored_at) {
    $lastBodyStoreMin = ((Get-Date) - [DateTimeOffset]::FromUnixTimeSeconds([int64]$rawSync.last_block_stored_at).LocalDateTime).TotalMinutes
}
$bodyIBDSnap = if ($info) { Get-DogeGoBodyIBDSnapshot $info } else { [ordered]@{} }
$bodyIBDPct = $bodyIBDSnap.body_pct
$bodyIBDEtaText = $bodyIBDSnap.body_eta_text
$bodyIBDEtaMin = if ($bodyIBDSnap.body_eta_min) { $bodyIBDSnap.body_eta_min } elseif ($bodyIBDSnap.body_eta_min_rpc) { $bodyIBDSnap.body_eta_min_rpc } else { $null }
$inFlightBatches = $bodyIBDSnap.in_flight
$utxoMemTip = $null
$utxoSnapshotDisk = $null
if ($info -and $info.PSObject.Properties.Name -contains "dogego_utxo_chain_active") {
    $utxoMemTip = [int64]$info.dogego_utxo_chain_active
}
if ($info -and $info.PSObject.Properties.Name -contains "dogego_utxo_snapshot_height") {
    $utxoSnapshotDisk = [int64]$info.dogego_utxo_snapshot_height
}
$snapshotReplayTarget = $null
if ($null -ne $utxoMemTip -and $utxoMemTip -ge 0) { $snapshotReplayTarget = $utxoMemTip }
elseif ($info -and $info.PSObject.Properties.Name -contains "dogego_utxo_replay_target") { $snapshotReplayTarget = [int64]$info.dogego_utxo_replay_target }
elseif ($null -ne $utxoSnapshotDisk -and $utxoSnapshotDisk -ge 0) { $snapshotReplayTarget = $utxoSnapshotDisk }
$snapshotBodyReplay = $false
if ($null -ne $snapshotReplayTarget -and $null -ne $cont -and $snapshotReplayTarget -gt ($cont + 1)) {
    $snapshotBodyReplay = $true
    $notes += "snapshot_body_replay_active"
    $remain = $snapshotReplayTarget - $cont
    if ($remain -gt 3000) {
        $warnings += "utxo_snapshot_far_ahead_of_bodies_$remain"
        $notes += "keep_node_running_until_body_replay_catches_up"
    }
    if ($info -and $info.PSObject.Properties.Name -contains "dogego_sync_phase" -and
        $info.dogego_sync_phase -eq "snapshot_body_replay") {
        $notes += "sync_phase_snapshot_body_replay"
    }
}
if ($info -and $info.PSObject.Properties.Name -contains "dogego_utxo_bodies_aligned" -and
    $info.dogego_utxo_bodies_aligned -eq $false) {
    $warnings += "utxo_bodies_not_aligned"
}

if ($info -and $info.PSObject.Properties.Name -contains "dogego_auxpow_parent_chain_id_core_parity" -and
    $info.dogego_auxpow_parent_chain_id_core_parity -ne $true) {
    $issues += "auxpow_parity_missing"
}
if ($info -and $info.PSObject.Properties.Name -contains "dogego_genesis_missing" -and $info.dogego_genesis_missing -eq $true) {
    if ($snapshotBodyReplay -and $null -ne $cont -and $cont -gt 0) {
        $warnings += "genesis_body_missing_reconcile_pending"
        $notes += "genesis_reconcile_expected_during_replay"
    } else {
        $issues += "genesis_body_missing"
    }
}
if ($info -and $info.PSObject.Properties.Name -contains "dogego_header_sync_recovery" -and
    $info.dogego_header_sync_recovery -match "outdated auxpow") {
    $issues += "auxpow_recovery_error"
}
if ($rawSync -and $null -ne $cont -and $rawSync.lowest_missing_height -gt ($cont + 1)) {
    $gapAhead = [int64]$rawSync.lowest_missing_height - $cont - 1
    if ($gapAhead -gt 0) {
        $warnings += "raw_body_ahead_of_contiguous_$gapAhead"
        $notes += "raw_body_gap_connect_capped"
    }
}
if ($headers -and $headers -lt 510000 -and ($info -or $web)) {
    $warnings += "headers_below_post_aux_510k"
}
if ($null -ne $connectLag -and $connectLag -gt 5000) {
    if ($connectRate -gt 25) {
        $notes += "connect_catchup_active"
    } elseif ($connectRate -ge 10) {
        $notes += "connect_catchup_slow"
    } elseif ($null -ne $blocks -and $blocks -lt 2000 -and (-not $connectRate -or $connectRate -lt 10)) {
        if (-not $snapshotBodyReplay) {
            $notes += "connect_catchup_warming"
        }
    } elseif ($downloadRate -gt 10 -or ($null -ne $lastBodyStoreMin -and $lastBodyStoreMin -lt 5)) {
        $notes += "connect_lag_expected_during_ibd"
    } elseif ($connectLag -gt 1000) {
        $notes += "connect_catchup_in_progress"
    } else {
        $warnings += "connect_lag_stalled"
    }
}
if ($disk.RawSyncMtime -and ((Get-Date) - $disk.RawSyncMtime).TotalMinutes -gt 15) {
    if ($null -eq $lastBodyStoreMin -or $lastBodyStoreMin -gt 15) {
        $warnings += "disk_raw_checkpoint_stale_15m"
    }
}
if ($null -ne $lastBodyStoreMin -and $lastBodyStoreMin -gt 10 -and $headers -and $cont -and ($headers - $cont) -gt 10000) {
    $atConnectFrontier = ($null -ne $blocks -and [int64]$blocks -eq [int64]$cont)
    if ($atConnectFrontier -and $connectRate -gt 5) {
        $notes += "connect_at_stored_frontier"
    } elseif ($null -eq $connectLag -or $connectLag -lt 500) {
        if (-not $atConnectFrontier) {
            $warnings += "body_download_stalled_10m"
        }
    } elseif ($connectRate -lt 20 -and ($null -eq $downloadRate -or $downloadRate -lt 5)) {
        $warnings += "body_download_slow_connect_and_fetch"
    }
} elseif ($null -ne $lastBodyStoreMin -and $lastBodyStoreMin -gt 5 -and ($null -eq $downloadRate -or $downloadRate -lt 1) -and $cont -and $headers -and ($headers - $cont) -gt 5000) {
    $warnings += "body_download_stalled_no_recent_store"
    $notes += "ibd_stall_recovery_should_run"
    if ($null -ne $assistPool -and $assistPool -eq 0) {
        $warnings += "assist_peer_pool_empty"
    } elseif ($null -ne $assistPool -and $assistPool -gt 0 -and ($null -eq $assistSessions -or $assistSessions -eq 0)) {
        $notes += "assist_pool_ready_no_sessions"
    }
} elseif ($null -ne $lastBodyStoreMin -and $lastBodyStoreMin -le 5 -and $downloadRate -gt 5) {
    $notes += "body_download_active"
}
if ($rpcUp -and -not $hasProcessLock) {
    $warnings += "process_lock_missing_restart_recommended"
}
if (-not $rpcUp -and $hasProcessLock) {
    $lockPath = Join-Path $disk.ChainDir ".dogego-process.lock"
    if (Test-Path $lockPath) {
        $lockText = Get-Content $lockPath -Raw -ErrorAction SilentlyContinue
        if ($lockText -match 'pid=(\d+)') {
            $lockPid = [int]$Matches[1]
            if (-not (Get-Process -Id $lockPid -ErrorAction SilentlyContinue)) {
                $issues += "stale_process_lock_pid_$lockPid"
                $notes += "run_restart_node_ps1"
            } else {
                $warnings += "rpc_down_process_may_be_hung"
            }
        }
    }
}
try {
    $webBase = (Get-DogeGoWebUIUrl).TrimEnd('/')
    $logLines = (Invoke-RestMethod -Uri ($webBase + "/api/logs?limit=300") -TimeoutSec 8).lines
    $connectBodyGapAt = $null
    $bodyGapMsgs = @($logLines | Where-Object { $_.msg -match 'connect height (\d+): raw body missing' })
    if ($bodyGapMsgs.Count -ge 2 -and $null -ne $blocks) {
        $lastGap = $bodyGapMsgs[-1]
        if ([string]$lastGap.msg -match 'connect height (\d+): raw body missing') {
            $gapH = [int64]$Matches[1]
            if ($gapH -eq ([int64]$blocks + 1)) {
                $gapRecent = $true
                if ($lastGap.ts) {
                    try {
                        $gapAt = [DateTimeOffset]::Parse([string]$lastGap.ts).UtcDateTime
                        $gapRecent = ((Get-Date).ToUniversalTime() - $gapAt).TotalMinutes -le 5
                    } catch { $gapRecent = $true }
                }
                if ($gapRecent) {
                    $storedAhead = ($null -ne $cont -and [int64]$cont -gt ([int64]$blocks + 1))
                    if (-not $storedAhead) {
                        $connectBodyGapAt = $gapH
                        $warnings += "connect_body_gap_at_$gapH"
                        $notes += "ibd_should_realign_to_height_$gapH"
                    } else {
                        $notes += "connect_catchup_behind_stored_bodies"
                    }
                }
            }
        }
    }
    $stallMsgs = @($logLines | Where-Object { $_.msg -match 'connect stalled at height (\d+)' })
    if ($stallMsgs.Count -ge 3) {
        $lastStall = $stallMsgs[-1]
        $lastMsg = [string]$lastStall.msg
        if ($lastMsg -match 'connect stalled at height (\d+)') {
            $stallH = [int64]$Matches[1]
            $stallRecent = $true
            if ($lastStall.ts) {
                try {
                    $stallAt = [DateTimeOffset]::Parse([string]$lastStall.ts).UtcDateTime
                    $stallRecent = ((Get-Date).ToUniversalTime() - $stallAt).TotalMinutes -le 10
                } catch { $stallRecent = $true }
            }
            if ($stallRecent -and $null -ne $blocks -and [int64]$blocks -eq $stallH) {
                if ($null -eq $connectBodyGapAt) {
                    $issues += "connect_chain_active_stuck_at_$stallH"
                    $notes += "check_web_logs_connect_height_$([int64]$stallH + 1)"
                } else {
                    $notes += "connect_stall_expected_until_body_gap_filled"
                }
            } elseif ($null -ne $blocks -and [int64]$blocks -gt $stallH) {
                $notes += "connect_stall_log_stale_at_$stallH"
            }
        }
    }
} catch {
    # Web UI optional when RPC-only monitoring.
}

if ($null -ne $connectLag -and $connectLag -gt 500 -and $connectRate -gt 5 -and $issues -contains "connect_chain_active_stuck_at_$blocks") {
    $warnings += "connect_rate_stale_lag_not_shrinking"
}

$healthy = ($issues.Count -eq 0) -and $rpcReady -and ($null -ne $headers) -and ($headers -ge 510000)
# During mainnet body IBD, forward progress matters more than strict RPC port reachability.
if (-not $healthy -and $rpcReady -and $issues.Count -eq 0 -and $null -ne $headers -and $headers -ge 510000) {
    $syncOk = $false
    if ($web -and $web.dogego_sync_health -match "forward_ibd|headers_catching|healthy") { $syncOk = $true }
    if ($null -ne $cont -and $cont -ge 1000 -and $null -ne $downloadRate -and $downloadRate -gt 0.5) { $syncOk = $true }
    if ($null -ne $lastBodyStoreMin -and $lastBodyStoreMin -le 10) { $syncOk = $true }
    if ($syncOk) { $healthy = $true; $notes += "healthy_for_mainnet_ibd" }
}

$out = [ordered]@{
    healthy                = $healthy
    rpc_up                 = $rpcUp
    rpc_ready              = $rpcReady
    headers                = $headers
    blocks                 = $blocks
    contiguous_raw         = $cont
    connect_lag            = $connectLag
    connect_blocks_per_min = $connectRate
    connect_catch_up_passes = $connectBoost.passes
    connect_catch_up_batch = $connectBoost.batch
    connect_catch_up_interval_ms = $connectBoost.interval_ms
    connect_catch_up_boost = $connectBoostLine
    download_blocks_per_min = $downloadRate
    utxo_connect_in_flight = $utxoConnectInFlight
    syncutxo_in_flight     = $syncUtxoInFlight
    utxo_snapshot_in_flight = $utxoSnapshotInFlight
    assist_peer_pool        = $assistPool
    assist_sessions         = $assistSessions
    last_body_store_min  = $lastBodyStoreMin
    in_flight_batches      = $inFlightBatches
    body_ibd_pct           = $bodyIBDPct
    body_ibd_eta_min       = $bodyIBDEtaMin
    body_ibd_eta           = $bodyIBDEtaText
    raw_probe              = $disk.RawProbe
    body_pct               = $disk.BodyPct
    issues                 = $issues
    warnings               = $warnings
    notes                  = $notes
    sync_health            = if ($web) { $web.dogego_sync_health } else { $null }
    process_lock           = $hasProcessLock
    body_ibd_paused        = if ($info) { $info.dogego_body_ibd_header_paused } elseif ($web) { $web.dogego_body_ibd_header_paused } else { $null }
    header_resume_blocks   = $bodyIBDSnap.header_resume_blocks
    header_resume_eta      = $bodyIBDSnap.header_resume_eta_text
    connect_catch_up_min_lag = if ($info -and $info.PSObject.Properties.Name -contains "dogego_connect_catch_up_min_lag") { [int64]$info.dogego_connect_catch_up_min_lag }
        elseif ($web -and $web.dogego_connect_catch_up_min_lag) { [int64]$web.dogego_connect_catch_up_min_lag } else { $null }
}

if ($Json) {
    $out | ConvertTo-Json -Compress
    if ($issues.Count -gt 0) { exit 2 }
    if (-not $healthy) { exit 1 }
    exit 0
}

Write-Host "=== DogeGo node health ===" -ForegroundColor Cyan
Write-Host ("RPC: {0}" -f $(if ($rpcReady) { "ready" } elseif ($rpcUp) { "warming up" } else { "down" }))
Write-Host ("process_lock: {0}" -f $(if ($hasProcessLock) { "active" } else { "missing" }))
Write-Host ("headers={0} blocks={1} stored={2} probe={3}" -f $headers, $blocks, $cont, $disk.RawProbe)
if ($snapshotBodyReplay -and $null -ne $cont) {
    $pct = [math]::Round(100.0 * ($cont + 1) / ($snapshotReplayTarget + 1), 1)
    $remain = $snapshotReplayTarget - $cont
    $replayLine = "snapshot_replay: bodies {0}/{1} ({2}% to utxo snapshot; ~{3} left)" -f $cont, $snapshotReplayTarget, $pct, $remain
    if ($downloadRate -and $downloadRate -gt 0 -and $remain -gt 0) {
        $replayEtaMin = [math]::Ceiling($remain / $downloadRate)
        if ($replayEtaMin -ge 120) {
            $replayLine += ("; ~{0:N1}h at current download rate" -f ($replayEtaMin / 60.0))
        } else {
            $replayLine += ("; ~{0}m at current download rate" -f $replayEtaMin)
        }
    }
    Write-Host $replayLine -ForegroundColor DarkGray
}
if ($null -ne $connectLag -and $connectLag -gt 0) {
    $lagLine = "connect_lag=$connectLag"
    if ($connectRate) {
        $lagLine += (" connect={0:N0}/min" -f $connectRate)
        $etaMin = [math]::Ceiling($connectLag / $connectRate)
        if ($etaMin -ge 60) {
            $lagLine += (" (~{0:N0}h to clear lag)" -f ($etaMin / 60.0))
        } else {
            $lagLine += (" (~{0}m to clear lag)" -f $etaMin)
        }
    }
    if ($downloadRate) { $lagLine += (" download={0:N0}/min" -f $downloadRate) }
    if ($utxoConnectInFlight -eq $true) { $lagLine += " utxo_connect=busy" }
    if ($syncUtxoInFlight -eq $true) { $lagLine += " syncutxo=busy" }
    if ($null -ne $assistPool) { $lagLine += (" assist_pool={0}" -f $assistPool) }
    if ($null -ne $assistSessions -and $assistSessions -gt 0) { $lagLine += (" assist_live={0}" -f $assistSessions) }
    if ($connectBoostLine) { $lagLine += (" boost={0}" -f $connectBoostLine) }
    Write-Host $lagLine -ForegroundColor DarkGray
}
if ($null -ne $headers -and $null -ne $cont -and $headers -gt $cont) {
    $bodyLine = "body_ibd: stored=$cont / headers=$headers"
    if ($bodyIBDPct) { $bodyLine += (" ({0}%)" -f $bodyIBDPct) }
    if ($downloadRate) { $bodyLine += (" download={0:N1}/min" -f $downloadRate) }
    if ($bodyIBDEtaText) { $bodyLine += (" ETA {0}" -f $bodyIBDEtaText) }
    if ($null -ne $bodyIBDSnap.header_resume_blocks) {
        $bodyLine += (" hdr_resume=$($bodyIBDSnap.header_resume_blocks)")
        if ($bodyIBDSnap.header_resume_eta_text) { $bodyLine += (" (~$($bodyIBDSnap.header_resume_eta_text))") }
    }
    if ($null -ne $inFlightBatches) { $bodyLine += (" in_flight=$inFlightBatches") }
    if ($null -ne $assistSessions -and $assistSessions -gt 0) { $bodyLine += (" assist=$assistSessions") }
    Write-Host $bodyLine -ForegroundColor DarkGray
}
if ($web -and $web.dogego_sync_status) { Write-Host $web.dogego_sync_status -ForegroundColor DarkGray }
foreach ($n in $notes) { Write-Host ("OK: {0}" -f $n) -ForegroundColor Green }
foreach ($w in $warnings) { Write-Host ("WARN: {0}" -f $w) -ForegroundColor Yellow }
foreach ($i in $issues) { Write-Host ("FAIL: {0}" -f $i) -ForegroundColor Red }
if ($healthy) {
    Write-Host "Overall: healthy for mainnet IBD" -ForegroundColor Green
    exit 0
}
if ($issues.Count -gt 0) {
    Write-Host "Overall: unhealthy" -ForegroundColor Red
    exit 2
}
Write-Host "Overall: ok (warnings only)" -ForegroundColor Yellow
exit 1
