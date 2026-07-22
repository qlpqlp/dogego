# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone B: mainnet live IBD soak gate - timed health + forward progress + auto-resume on crash.
#
#   .\scripts\ibd_live_soak_gate.ps1
#   .\scripts\ibd_live_soak_gate.ps1 -DurationMin 30 -IntervalSec 120
#   $env:DOGEGO_IBD_LIVE_SOAK = "1"; .\scripts\core_operator_workflow_cert.ps1
param(
    [int]$DurationMin = 20,
    [int]$IntervalSec = 120,
    [string]$DataDir = "dogedata",
    [string]$Network = "mainnet"
)
$ErrorActionPreference = "Stop"

Write-Host "=== Mainnet IBD live soak gate (Milestone B) ===" -ForegroundColor Cyan
Write-Host "Proves forward body IBD progress over a timed window with crash auto-resume." -ForegroundColor DarkGray

& "$PSScriptRoot\ibd_timed_soak.ps1" `
    -DurationMin $DurationMin `
    -IntervalSec $IntervalSec `
    -RequireConvergence `
    -AutoRestartOnStaleLock `
    -DataDir $DataDir `
    -Network $Network

exit $LASTEXITCODE
