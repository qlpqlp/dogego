# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: verify restart resume - checkpoint aligns with contiguous bodies and assist pool during IBD.
# Requires a running DogeGo node on mainnet (or pass -Network testnet).
#
#   .\scripts\core_restart_resume_check.ps1
#   .\scripts\core_restart_resume_check.ps1 -Json
param(
    [switch]$Json,
    [string]$DataDir,
    [string]$Network = "mainnet",
    [switch]$SkipAutostartCheck
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

$conf = Read-DogeGoConfig
$disk = Get-DogeGoDiskSyncSnapshot -DataDir $DataDir -Network $Network
$issues = @()
$warnings = @()
$notes = @()
$info = $null

try {
    $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 5 -WarmupDelaySec 2
} catch {
    $issues += "rpc_unreachable"
}

$cont = $null
$headers = $null
$assistPool = $null
$assistSessions = $null
$checkpointProbe = $null

if ($info) {
    $headers = [int64]$info.headers
    if ($info.PSObject.Properties.Name -contains "dogego_contiguous_raw_height") {
        $cont = [int64]$info.dogego_contiguous_raw_height
    }
    if ($info.PSObject.Properties.Name -contains "dogego_raw_sync" -and $info.dogego_raw_sync) {
        $rs = $info.dogego_raw_sync
        if ($rs.PSObject.Properties.Name -contains "assist_peer_pool") {
            $assistPool = [int]$rs.assist_peer_pool
        }
        if ($rs.PSObject.Properties.Name -contains "assist_active_sessions") {
            $assistSessions = [int]$rs.assist_active_sessions
        }
    }
}

if ($null -ne $disk.RawProbe) {
    $checkpointProbe = [int64]$disk.RawProbe
}

if ($null -ne $cont -and $null -ne $checkpointProbe -and $checkpointProbe -gt ($cont + 64)) {
    $warnings += "checkpoint_ahead_of_contiguous"
    $notes += "initProgressiveRawAtStartup_should_realign"
}

if ($null -ne $cont -and $cont -lt 0 -and $headers -gt 1000) {
    $warnings += "no_contiguous_bodies_yet"
}

$bodyLag = $null
if ($null -ne $headers -and $null -ne $cont -and $headers -gt $cont) {
    $bodyLag = $headers - $cont
}

if ($info -and $info.initialblockdownload -eq $true -and $null -ne $bodyLag -and $bodyLag -gt 5000) {
    if ($null -eq $assistPool -or $assistPool -eq 0) {
        $issues += "assist_peer_pool_empty_during_ibd"
    } elseif ($null -eq $assistSessions -or $assistSessions -eq 0) {
        $warnings += "assist_pool_ready_no_active_sessions"
    } else {
        $notes += "assist_ibd_healthy"
    }
}

$connectLag = $null
$connectBoost = $null
if ($info) {
    $connectLag = Get-DogeGoRpcConnectLag $info
    $connectBoost = Format-DogeGoConnectCatchUpBoost (Get-DogeGoRpcConnectCatchUpTuning $info)
    if ($null -ne $connectLag -and $connectLag -gt 128) {
        if ($info.initialblockdownload -eq $true) {
            $warnings += "connect_lag_above_threshold"
        } else {
            $issues += "connect_lag_above_threshold"
        }
    }
}

try {
    $zmq = Invoke-DogeGoJsonRpc -Method getzmqnotifications
    if ($null -eq $zmq) {
        $warnings += "getzmqnotifications_null"
    }
} catch {
    if ($_.Exception.Message -match "not implemented|32601") {
        $issues += "getzmqnotifications_missing"
    } else {
        $warnings += "getzmqnotifications_error"
    }
}

$autostartWant = $false
$autostartInstalled = $false
$autostartMethod = $null
if (-not $SkipAutostartCheck) {
    $as = Test-DogeGoAutostartGate -Conf $conf
    $autostartWant = [bool]$as.want
    if ($as.skipped) {
        $notes += $as.note
    } elseif ($as.warning) {
        $warnings += $as.warning
        if ($as.detail) { $notes += $as.detail }
    } elseif (-not $as.ok) {
        $issues += $as.issue
    } else {
        $autostartInstalled = [bool]$as.installed
        $autostartMethod = $as.method
        $notes += "autostart_registered"
        if ($as.detail) { $notes += $as.detail }
    }
}

$ok = ($issues.Count -eq 0)
$report = [ordered]@{
    ok                  = $ok
    network             = $Network
    headers             = $headers
    contiguous_raw      = $cont
    checkpoint_probe    = $checkpointProbe
    body_lag            = $bodyLag
    assist_peer_pool    = $assistPool
    assist_sessions     = $assistSessions
    connect_lag         = $connectLag
    connect_catch_up_boost = $connectBoost
    autostart_want      = $autostartWant
    autostart_installed = $autostartInstalled
    autostart_method    = $autostartMethod
    issues              = @($issues)
    warnings            = @($warnings)
    notes               = @($notes)
}

if ($Json) {
    $report | ConvertTo-Json -Depth 5
} else {
    Write-Host "=== Core restart resume check ===" -ForegroundColor Cyan
    Write-Host ("headers={0} contiguous={1} checkpoint_probe={2}" -f $headers, $cont, $checkpointProbe)
    if ($assistPool -ne $null) {
        Write-Host ("assist_pool={0} sessions={1}" -f $assistPool, $assistSessions)
    }
    if ($null -ne $connectLag) {
        $lagLine = "connect_lag=$connectLag"
        if ($connectBoost) { $lagLine += " boost=$connectBoost" }
        Write-Host $lagLine -ForegroundColor DarkGray
    }
    if ($autostartWant) {
        Write-Host ("autostart=login installed={0} method={1}" -f $autostartInstalled, $autostartMethod)
    }
    foreach ($w in $warnings) { Write-Host ("WARN: " + $w) -ForegroundColor Yellow }
    foreach ($i in $issues) { Write-Host ("FAIL: " + $i) -ForegroundColor Red }
    foreach ($n in $notes) { Write-Host ("NOTE: " + $n) -ForegroundColor DarkGray }
    if ($ok) {
        Write-Host "`nRestart resume check passed." -ForegroundColor Green
    } else {
        Write-Host "`nRestart resume check failed." -ForegroundColor Red
    }
}

if (-not $ok) { exit 1 }
exit 0
