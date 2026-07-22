# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone B: crash/corruption + timed IBD soak certification (tiered).
#
# Tiers:
#   offline     - kill/recovery Go tests + corruption_soak_cert + operator_workflow (no live node)
#   mini        - reboottestnet: corruption_extended_cert_mini (~15-25 min)
#   live        - reboottestnet: ci_live_reboottestnet_gate (health + E2E + Core 24/24 + corruption mini)
#   long        - reboottestnet: corruption_long_soak_gate (45+ min timed inject + health pre-soak)
#   extended    - reboottestnet: extended_operator_soak (timed IBD + corruption cycles)
#   mainnet-ibd - mainnet: ibd_live_soak_gate (forward body IBD progress; requires running node)
#   full        - reboottestnet: long soak + extended mini verify (CI Milestone B exit candidate)
#
# Usage:
#   .\scripts\milestone_b_cert.ps1
#   .\scripts\milestone_b_cert.ps1 -Tier mini
#   .\scripts\milestone_b_cert.ps1 -Tier mainnet-ibd -DurationMin 30
#   $env:DOGEGO_MILESTONE_B_TIER = "long"; .\scripts\milestone_b_cert.ps1
param(
    [ValidateSet("offline", "mini", "live", "long", "extended", "mainnet-ibd", "full")]
    [string]$Tier = "",
    [int]$DurationMin = 0,
    [int]$IntervalSec = 120,
    [string]$DataDir = "dogedata",
    [string]$Network = "",
    [switch]$Json
)
$ErrorActionPreference = "Stop"

if (-not $Tier) {
    if ($env:DOGEGO_MILESTONE_B_TIER) {
        $Tier = $env:DOGEGO_MILESTONE_B_TIER
    } else {
        $Tier = "offline"
    }
}

$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

$steps = @()
$failed = $false

function Step {
    param([string]$Name, [string]$Script, [hashtable]$StepArgs = @{})
    Write-Host "`n=== Milestone B [$Tier]: $Name ===" -ForegroundColor Cyan
    & $Script @StepArgs
    $ok = ($LASTEXITCODE -eq 0)
    $script:steps += [ordered]@{ name = $Name; ok = $ok; exit = $LASTEXITCODE }
    if (-not $ok) { $script:failed = $true }
    return $ok
}

Write-Host "=== Milestone B certification (tier=$Tier) ===" -ForegroundColor Cyan

switch ($Tier) {
    "offline" {
        Step "corruption_soak_cert" "$PSScriptRoot\corruption_soak_cert.ps1" @{}
        if (-not $failed) {
            Write-Host "For full offline CI gate (Milestone E corpus): .\scripts\ci_offline_gate.ps1" -ForegroundColor DarkGray
        }
    }
    "mini" {
        $net = if ($Network) { $Network } else { "reboottestnet" }
        if ($net -eq "mainnet") { Write-Error "Tier mini is reboottestnet-only." }
        Step "corruption_extended_cert_mini" "$PSScriptRoot\corruption_extended_cert_mini.ps1" @{
            Network = $net
            DataDir = $DataDir
        }
    }
    "live" {
        $net = if ($Network) { $Network } else { "reboottestnet" }
        if ($net -eq "mainnet") { Write-Error "Tier live is reboottestnet-only." }
        Step "ci_live_reboottestnet_gate" "$PSScriptRoot\ci_live_reboottestnet_gate.ps1" @{
            IncludeCoreAlignedGate = $true
            IncludeCorruptionMini  = $true
            Network                = $net
            DataDir                = $DataDir
        }
    }
    "long" {
        $net = if ($Network) { $Network } else { "reboottestnet" }
        if ($net -eq "mainnet") { Write-Error "Tier long is reboottestnet-only." }
        $longArgs = @{ Network = $net; DataDir = $DataDir }
        if ($DurationMin -gt 0) { $longArgs.DurationMin = $DurationMin }
        Step "corruption_long_soak_gate" "$PSScriptRoot\corruption_long_soak_gate.ps1" $longArgs
    }
    "extended" {
        $net = if ($Network) { $Network } else { "reboottestnet" }
        if ($net -eq "mainnet") { Write-Error "Tier extended is reboottestnet-only." }
        $dur = if ($DurationMin -gt 0) { $DurationMin } else { 20 }
        Step "extended_operator_soak" "$PSScriptRoot\extended_operator_soak.ps1" @{
            DurationMin          = $dur
            IntervalSec          = $IntervalSec
            Network              = $net
            DataDir              = $DataDir
            CorruptionCycles     = 2
            TimedCorruptionLoop  = $true
            LoopDurationMin      = [Math]::Max(15, [int]($dur * 0.75))
        }
    }
    "mainnet-ibd" {
        $net = if ($Network) { $Network } else { "mainnet" }
        if ($net -ne "mainnet") { Write-Error "Tier mainnet-ibd requires Network=mainnet." }
        $dur = if ($DurationMin -gt 0) { $DurationMin } else { 20 }
        Step "ibd_live_soak_gate" "$PSScriptRoot\ibd_live_soak_gate.ps1" @{
            DurationMin  = $dur
            IntervalSec  = $IntervalSec
            DataDir      = $DataDir
            Network      = $net
        }
    }
    "full" {
        $net = if ($Network) { $Network } else { "reboottestnet" }
        if ($net -eq "mainnet") { Write-Error "Tier full is reboottestnet-only." }
        Step "ci_milestone_b_full_gate" "$PSScriptRoot\ci_milestone_b_full_gate.ps1" @{
            DurationMin = $(if ($DurationMin -gt 0) { $DurationMin } else { 0 })
            DataDir     = $DataDir
            Network     = $net
        }
    }
    default {
        Write-Error "Unknown tier: $Tier"
    }
}

$report = [ordered]@{
    tier   = $Tier
    ok     = (-not $failed)
    steps  = $steps
}

if ($Json) {
    $report | ConvertTo-Json -Depth 5
} elseif ($failed) {
    Write-Host "`nMilestone B certification FAILED (tier=$Tier)." -ForegroundColor Red
    foreach ($s in $steps) {
        if (-not $s.ok) { Write-Host ("  FAIL: $($s.name) exit=$($s.exit)") -ForegroundColor Red }
    }
    exit 1
} else {
    Write-Host "`nMilestone B certification passed (tier=$Tier)." -ForegroundColor Green
}

if ($failed) { exit 1 }
exit 0
