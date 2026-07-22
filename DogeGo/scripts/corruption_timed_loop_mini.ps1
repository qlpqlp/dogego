# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone B: short timed corruption loop for operator cert (reboottestnet default).
# DISRUPTIVE - one inject cycle per round, verifychain after each round.
#
#   .\scripts\corruption_timed_loop_mini.ps1
#   $env:DOGEGO_CORRUPTION_TIMED_LOOP = "1"; .\scripts\core_operator_workflow_cert.ps1
param(
    [int]$DurationMin = 8,
    [int]$IntervalMin = 2,
    [int]$CorruptionCycles = 1,
    [string[]]$Targets = @("headers", "raw"),
    [string]$DataDir = "dogedata",
    [string]$Network = "reboottestnet",
    [int]$WaitSec = 45
)
$ErrorActionPreference = "Stop"

if ($Network -eq "mainnet") {
    Write-Error "corruption_timed_loop_mini is reboottestnet-only (use -Network reboottestnet)."
}

Write-Host "=== Corruption timed loop (mini cert, ${DurationMin}m) ===" -ForegroundColor Cyan

& "$PSScriptRoot\corruption_timed_loop.ps1" @PSBoundParameters
exit $LASTEXITCODE
