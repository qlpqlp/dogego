# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone B: timed corruption inject loop with convergence assertions (reboottestnet default).
# DISRUPTIVE - repeats corruption_inject_soak until duration elapses.
#
#   .\scripts\corruption_timed_loop.ps1 -DurationMin 60 -IntervalMin 15
#   .\scripts\corruption_timed_loop.ps1 -Targets headers,raw -CorruptionCycles 2
param(
    [int]$DurationMin = 60,
    [int]$IntervalMin = 10,
    [int]$CorruptionCycles = 1,
    [string[]]$Targets = @("headers", "raw", "filter", "txindex"),
    [string]$DataDir = "dogedata",
    [string]$Network = "reboottestnet",
    [switch]$AllowMainnet,
    [int]$WaitSec = 60
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

if ($DurationMin -lt 5) { throw "DurationMin must be >= 5" }
if ($IntervalMin -lt 2) { throw "IntervalMin must be >= 2" }
if ($CorruptionCycles -lt 1) { throw "CorruptionCycles must be >= 1" }

$deadline = (Get-Date).AddMinutes($DurationMin)
$round = 0
$summary = @()

Write-Host "=== Timed corruption loop ($DurationMin min, interval ${IntervalMin}m, cycles=$CorruptionCycles) ===" -ForegroundColor Cyan
Write-Host "Targets: $($Targets -join ', ')  Network: $Network" -ForegroundColor DarkGray

while ((Get-Date) -lt $deadline) {
    $round++
    Write-Host "`n=== Round $round @ $(Get-Date -Format 'HH:mm:ss') ===" -ForegroundColor Cyan

    $beforeBlocks = $null
    try {
        $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 2 -WarmupDelaySec 2
        $beforeBlocks = [int64]$info.blocks
    } catch {
        Write-Warning "getblockchaininfo before round $round`: $_"
    }

    $soakArgs = @{
        Targets  = $Targets
        DataDir  = $DataDir
        Network  = $Network
        WaitSec  = $WaitSec
    }
    if ($AllowMainnet) { $soakArgs.AllowMainnet = $true }

    for ($cycle = 1; $cycle -le $CorruptionCycles; $cycle++) {
        if ($CorruptionCycles -gt 1) {
            Write-Host "Corruption cycle $cycle / $CorruptionCycles" -ForegroundColor DarkGray
        }
        & "$PSScriptRoot\corruption_inject_soak.ps1" @soakArgs
        if ($LASTEXITCODE -ne 0) {
            Write-Error "corruption inject soak failed on round $round cycle $cycle"
        }
        try {
            $vc = Invoke-DogeGoJsonRpc -Method verifychain -Params @(2, 0) -WarmupRetries 2 -WarmupDelaySec 2
            if ($null -eq $vc -or $vc -eq $false) {
                Write-Error "verifychain 2 0 failed on round $round cycle $cycle"
            }
        } catch {
            Write-Error "verifychain on round $round cycle $cycle`: $_"
        }
    }

    $afterBlocks = $null
    try {
        $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 2 -WarmupDelaySec 2
        $afterBlocks = [int64]$info.blocks
    } catch {
        Write-Error "getblockchaininfo after round $round`: $_"
    }

    if ($null -ne $beforeBlocks -and $afterBlocks -lt ($beforeBlocks - 2)) {
        Write-Error "block height regressed on round $round`: before=$beforeBlocks after=$afterBlocks"
    }

    $summary += [pscustomobject]@{
        Round        = $round
        BlocksBefore = $beforeBlocks
        BlocksAfter  = $afterBlocks
        Time         = (Get-Date).ToString("o")
    }

    $remaining = ($deadline - (Get-Date)).TotalMinutes
    if ($remaining -lt $IntervalMin) {
        Write-Host "Remaining $([math]::Round($remaining, 1))m < interval ${IntervalMin}m - done." -ForegroundColor DarkGray
        break
    }
    Write-Host "Sleeping ${IntervalMin}m before next round ($([math]::Round($remaining, 1))m left)..." -ForegroundColor DarkGray
    Start-Sleep -Seconds ($IntervalMin * 60)
}

Write-Host "`nTimed corruption loop summary ($round rounds):" -ForegroundColor DarkGray
$summary | Format-Table -AutoSize | Out-String | Write-Host
Write-Host "Timed corruption loop passed." -ForegroundColor Green
exit 0
