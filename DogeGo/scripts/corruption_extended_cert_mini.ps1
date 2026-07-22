# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone B: short extended cert - timed health + corruption loop on raw/index/filter (reboottestnet).
#
#   .\scripts\corruption_extended_cert_mini.ps1
#   $env:DOGEGO_CORRUPTION_EXTENDED_MINI = "1"; .\scripts\core_operator_workflow_cert.ps1
param(
    [int]$HealthDurationMin = 5,
    [int]$LoopDurationMin = 10,
    [int]$LoopIntervalMin = 2,
    [string[]]$Targets = @("headers", "raw", "filter", "txindex"),
    [string]$DataDir = "dogedata",
    [string]$Network = "reboottestnet"
)
$ErrorActionPreference = "Stop"

if ($Network -eq "mainnet") {
    Write-Error "corruption_extended_cert_mini is reboottestnet-only."
}

Write-Host "=== Extended corruption cert mini (health + timed loop) ===" -ForegroundColor Cyan

& "$PSScriptRoot\ibd_timed_soak.ps1" @{
    DurationMin = $HealthDurationMin
    IntervalSec = 60
    DataDir     = $DataDir
    Network     = $Network
}
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

& "$PSScriptRoot\corruption_timed_loop.ps1" @{
    DurationMin      = $LoopDurationMin
    IntervalMin      = $LoopIntervalMin
    CorruptionCycles = 1
    Targets          = $Targets
    DataDir          = $DataDir
    Network          = $Network
    WaitSec          = 45
}
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "`nExtended corruption cert mini passed." -ForegroundColor Green
exit 0
