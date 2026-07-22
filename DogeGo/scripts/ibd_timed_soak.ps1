# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone B (partial): timed IBD soak - repeated health + optional convergence over a window.
#
#   .\scripts\ibd_timed_soak.ps1 -DurationMin 30 -IntervalSec 120
#   .\scripts\ibd_timed_soak.ps1 -DurationMin 10 -RequireConvergence
param(
    [int]$DurationMin = 15,
    [int]$IntervalSec = 120,
    [switch]$RequireConvergence,
    [switch]$AutoRestartOnStaleLock,
    [switch]$Json,
    [string]$DataDir,
    [string]$Network = "mainnet"
)
$ErrorActionPreference = "Stop"

if ($DurationMin -lt 1) { throw "DurationMin must be >= 1" }
if ($IntervalSec -lt 30) { throw "IntervalSec must be >= 30" }

$deadline = (Get-Date).AddMinutes($DurationMin)
$passes = 0
$fails = 0
$converged = $false
$samples = @()

Write-Host ("=== IBD timed soak ({0} min, interval {1}s) ===" -f $DurationMin, $IntervalSec) -ForegroundColor Cyan
if ($AutoRestartOnStaleLock) {
    Write-Host "AutoRestartOnStaleLock: enabled (resume after stale process lock)" -ForegroundColor DarkGray
}

while ((Get-Date) -lt $deadline) {
    $ts = (Get-Date).ToString("o")
    $healthArgs = @{}
    if ($DataDir) { $healthArgs.DataDir = $DataDir }
    if ($Network) { $healthArgs.Network = $Network }

    if ($AutoRestartOnStaleLock) {
        $monitorArgs = @{ ConvergeSec = 0 }
        if ($DataDir) { $monitorArgs.DataDir = $DataDir }
        if ($Network) { $monitorArgs.Network = $Network }
        $monitorArgs.AutoRestartOnStaleLock = $true
        & "$PSScriptRoot\ibd_monitor.ps1" @monitorArgs | Out-Null
    }

    & "$PSScriptRoot\node_health.ps1" @healthArgs
    $healthExit = $LASTEXITCODE
    $convOk = $null
    if ($RequireConvergence -or $healthExit -lt 2) {
        $convParams = @{
            IntervalSec          = [Math]::Min(90, $IntervalSec)
            MinBlocksAdvance     = 0
            MinContiguousAdvance = 0
            MinRawProbeAdvance   = 1
            Network              = $Network
        }
        if ($DataDir) { $convParams.DataDir = $DataDir }
        & "$PSScriptRoot\ibd_convergence_check.ps1" @convParams | Out-Null
        $convOk = ($LASTEXITCODE -eq 0)
        if ($convOk) { $converged = $true }
    }

    $ok = ($healthExit -lt 2)
    if ($ok) { $passes++ } else { $fails++ }
    $samples += [ordered]@{
        timestamp    = $ts
        health_exit  = $healthExit
        convergence  = $convOk
    }

    if ((Get-Date) -ge $deadline) { break }
    Start-Sleep -Seconds $IntervalSec
}

$allOk = ($fails -eq 0) -and ((-not $RequireConvergence) -or $converged)
$report = [ordered]@{
    ok           = $allOk
    duration_min = $DurationMin
    interval_sec = $IntervalSec
    passes       = $passes
    fails        = $fails
    converged    = $converged
    samples      = $samples
}

if ($Json) {
    $report | ConvertTo-Json -Depth 5
} else {
    Write-Host ("passes={0} fails={1} converged={2}" -f $passes, $fails, $converged)
    if ($allOk) {
        Write-Host "`nTimed IBD soak passed." -ForegroundColor Green
    } else {
        Write-Host "`nTimed IBD soak failed." -ForegroundColor Red
    }
}

if (-not $allOk) { exit 1 }
exit 0
