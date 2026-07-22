# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone B: extended timed corruption soak (reboottestnet default).
# DISRUPTIVE - multi-round inject on headers/raw/filter/txindex with verifychain each cycle.
#
#   .\scripts\corruption_long_soak_gate.ps1
#   $env:DOGEGO_CORRUPTION_LONG_SOAK = "1"; .\scripts\core_operator_workflow_cert.ps1
#   $env:DOGEGO_CORRUPTION_LONG_MIN = "30"  # override default 45m
param(
    [int]$DurationMin = 0,
    [int]$IntervalMin = 8,
    [int]$CorruptionCycles = 2,
    [int]$HealthDurationMin = 10,
    [string[]]$Targets = @("headers", "raw", "filter", "txindex"),
    [string]$DataDir = "dogedata",
    [string]$Network = "reboottestnet"
)
$ErrorActionPreference = "Stop"

if ($Network -eq "mainnet") {
    Write-Error "corruption_long_soak_gate is reboottestnet-only."
}

if ($DurationMin -le 0) {
  if ($env:DOGEGO_CORRUPTION_LONG_MIN) {
    $DurationMin = [int]$env:DOGEGO_CORRUPTION_LONG_MIN
  } else {
    $DurationMin = 45
  }
}
if ($DurationMin -lt 15) {
    Write-Error "DurationMin must be >= 15 (set DOGEGO_CORRUPTION_LONG_MIN for shorter dev runs)."
}

Write-Host "=== Corruption long soak gate (${DurationMin}m, interval ${IntervalMin}m) ===" -ForegroundColor Cyan

& "$PSScriptRoot\ibd_timed_soak.ps1" -DurationMin $HealthDurationMin -IntervalSec 90 -DataDir $DataDir -Network $Network
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

& "$PSScriptRoot\corruption_timed_loop.ps1" -DurationMin $DurationMin -IntervalMin $IntervalMin -CorruptionCycles $CorruptionCycles -Targets $Targets -DataDir $DataDir -Network $Network -WaitSec 60
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "`nCorruption long soak gate passed." -ForegroundColor Green
exit 0
