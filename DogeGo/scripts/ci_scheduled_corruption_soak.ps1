# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone B: scheduled / CI entry for corruption long soak (reboottestnet live node required).
#
# Cross-platform: dogego cert live-soak
#   go run ./cmd/dogego cert live-soak -duration-min 60
#   go run ./cmd/dogego cert live-soak -require-soak-env
#
# Runbook: docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md (workflow 10)
#
#   .\scripts\ci_scheduled_corruption_soak.ps1
#   $env:DOGEGO_CORRUPTION_LONG_MIN = "60"; .\scripts\ci_scheduled_corruption_soak.ps1
param(
    [int]$DurationMin = 0,
    [string]$Network = "reboottestnet",
    [string]$DataDir = "dogedata"
)
$ErrorActionPreference = "Stop"

if ($Network -eq "mainnet") {
    Write-Error "Scheduled corruption soak is reboottestnet-only."
}

Write-Host "=== CI scheduled corruption soak ===" -ForegroundColor Cyan
Write-Host "Delegating to ci_milestone_b_full_gate.ps1" -ForegroundColor DarkGray

$fullArgs = @{ Network = $Network; DataDir = $DataDir }
if ($DurationMin -gt 0) { $fullArgs.DurationMin = $DurationMin }

& "$PSScriptRoot\ci_milestone_b_full_gate.ps1" @fullArgs
exit $LASTEXITCODE
