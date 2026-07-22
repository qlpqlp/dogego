# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone B (full): multi-hour reboottestnet soak - health pre-check, corruption long soak, post verifychain.
# Intended for dogego-live self-hosted runner + DOGEGO_SCHEDULED_LIVE_SOAK=1.
#
# Cross-platform: dogego cert live-soak
#   go run ./cmd/dogego cert live-soak -duration-min 60
#   go run ./cmd/dogego cert live-soak -skip-scripts   # preflight-only
#
# Runbook: docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md (workflow 10)
#
#   .\scripts\ci_milestone_b_full_gate.ps1
#   .\scripts\ci_milestone_b_full_gate.ps1 -DurationMin 60
param(
    [int]$DurationMin = 0,
    [string]$Network = "reboottestnet",
    [string]$DataDir = "dogedata",
    [string]$DogeGoRpcPort = "44556"
)
$ErrorActionPreference = "Stop"

if ($Network -eq "mainnet") {
    Write-Error "Milestone B full gate is reboottestnet-only."
}

Write-Host "=== CI Milestone B full gate ===" -ForegroundColor Cyan

$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

go test ./docs/... ./ui/... -count=1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
go test ./consensus -run 'TestStatefulMempool' -count=1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

& "$PSScriptRoot\ci_runner_preflight.ps1" -RequireCore -DogeGoRpcPort $DogeGoRpcPort
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

& "$PSScriptRoot\node_health.ps1" -Network $Network -DataDir $DataDir
if ($LASTEXITCODE -ge 2) { exit 2 }

if ($DurationMin -le 0) {
    if ($env:DOGEGO_CORRUPTION_LONG_MIN) {
        $DurationMin = [int]$env:DOGEGO_CORRUPTION_LONG_MIN
    } else {
        $DurationMin = 45
    }
}

$env:DOGEGO_CORRUPTION_LONG_SOAK = "1"
$env:DOGEGO_CORRUPTION_LONG_MIN = "$DurationMin"

$longArgs = @{
    DurationMin = $DurationMin
    Network     = $Network
    DataDir     = $DataDir
}
& "$PSScriptRoot\corruption_long_soak_gate.ps1" @longArgs
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

. "$PSScriptRoot\dogego_rpc.ps1"
try {
    $vc = Invoke-DogeGoJsonRpc -Method verifychain -Params @(4, 0) -WarmupRetries 3 -WarmupDelaySec 2 -TimeoutSec 120
    if ($null -eq $vc -or $vc -eq $false) {
        Write-Host "FAIL: verifychain 4 0 after Milestone B full soak" -ForegroundColor Red
        exit 1
    }
    Write-Host "verifychain 4 0: ok" -ForegroundColor DarkGray
} catch {
    Write-Host "FAIL: verifychain after soak: $_" -ForegroundColor Red
    exit 1
}

& "$PSScriptRoot\ibd_convergence_check.ps1" -IntervalSec 90 -MinRawProbeAdvance 1 -Network $Network -DataDir $DataDir
if ($LASTEXITCODE -ne 0) {
    Write-Host "WARN: post-soak convergence check did not show forward progress (node may be caught up)" -ForegroundColor Yellow
}

Write-Host "`nCI Milestone B full gate passed (${DurationMin}m corruption soak + verifychain)." -ForegroundColor Green
exit 0
