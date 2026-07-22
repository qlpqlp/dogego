# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E (partial): timed BIP152 HB + cmpct relay soak - repeated core_bip152_probe over a window.
#
#   .\scripts\bip152_timed_soak.ps1
#   .\scripts\bip152_timed_soak.ps1 -DurationMin 20 -IntervalSec 90 -RequireRelayActivity
param(
    [int]$DurationMin = 15,
    [int]$IntervalSec = 60,
    [int]$RpcPort = 0,
    [switch]$RequireRelayActivity,
    [switch]$Json
)
$ErrorActionPreference = "Stop"

if ($DurationMin -lt 1) { throw "DurationMin must be >= 1" }
if ($IntervalSec -lt 30) { throw "IntervalSec must be >= 30" }

$deadline = (Get-Date).AddMinutes($DurationMin)
$passes = 0
$fails = 0
$relayActive = $false
$baselineCmpct = $null
$lastCmpct = $null
$samples = @()

Write-Host ("=== BIP152 timed soak ({0} min, interval {1}s) ===" -f $DurationMin, $IntervalSec) -ForegroundColor Cyan
if ($RequireRelayActivity) {
    Write-Host "RequireRelayActivity: cmpct counters must advance or relay_active when HB peers present" -ForegroundColor DarkGray
}

while ((Get-Date) -lt $deadline) {
    $ts = (Get-Date).ToString("o")
    $probeArgs = @{ Json = $true }
    if ($RpcPort -gt 0) { $probeArgs.RpcPort = $RpcPort }
    $raw = & "$PSScriptRoot\core_bip152_probe.ps1" @probeArgs
    $probeExit = $LASTEXITCODE
    $row = $null
    try {
        $row = $raw | ConvertFrom-Json
    } catch {
        $row = $null
    }

    $ok = ($probeExit -eq 0) -and $row -and $row.ok -eq $true
    if ($ok) { $passes++ } else { $fails++ }

    $cmpctSum = 0
    if ($row -and $row.cmpct_relay) {
        foreach ($k in $row.cmpct_relay.PSObject.Properties.Name) {
            $cmpctSum += [int64]$row.cmpct_relay.$k
        }
        if ($null -eq $baselineCmpct) { $baselineCmpct = $cmpctSum }
        $lastCmpct = $cmpctSum
    }
    if ($row -and $row.notes) {
        foreach ($n in @($row.notes)) {
            if ("$n" -match "cmpct_relay_active") { $relayActive = $true }
        }
    }
    if ($row -and ($row.hb_to_peers -gt 0 -or $row.hb_from_peers -gt 0) -and $cmpctSum -gt 0) {
        $relayActive = $true
    }

    $samples += [ordered]@{
        timestamp           = $ts
        ok                  = $ok
        peer_count          = if ($row) { $row.peer_count } else { $null }
        hb_to_peers         = if ($row) { $row.hb_to_peers } else { $null }
        hb_from_peers       = if ($row) { $row.hb_from_peers } else { $null }
        cmpct_relay_schema_ok = if ($row) { $row.cmpct_relay_schema_ok } else { $null }
        cmpct_sum           = $cmpctSum
    }

    if ((Get-Date) -ge $deadline) { break }
    Start-Sleep -Seconds $IntervalSec
}

$cmpctAdvanced = ($null -ne $baselineCmpct) -and ($null -ne $lastCmpct) -and ($lastCmpct -gt $baselineCmpct)
$relayOk = (-not $RequireRelayActivity) -or $relayActive -or $cmpctAdvanced

$allOk = ($fails -eq 0) -and $relayOk
$report = [ordered]@{
    ok                 = $allOk
    duration_min       = $DurationMin
    interval_sec       = $IntervalSec
    passes             = $passes
    fails              = $fails
    relay_active       = $relayActive
    cmpct_advanced     = $cmpctAdvanced
    require_relay      = [bool]$RequireRelayActivity
    samples            = $samples
}

if ($Json) {
    $report | ConvertTo-Json -Depth 6
} else {
    Write-Host ("passes={0} fails={1} relay_active={2} cmpct_advanced={3}" -f $passes, $fails, $relayActive, $cmpctAdvanced)
    if ($allOk) {
        Write-Host "`nBIP152 timed soak passed." -ForegroundColor Green
    } else {
        if ($fails -gt 0) { Write-Host "Probe failures during window." -ForegroundColor Red }
        if (-not $relayOk) { Write-Host "RequireRelayActivity: no cmpct relay activity observed." -ForegroundColor Red }
        Write-Host "`nBIP152 timed soak failed." -ForegroundColor Red
    }
}

if (-not $allOk) { exit 1 }
exit 0
