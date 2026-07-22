# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone B/D/E: live reboottestnet CI gate (self-hosted dogego-live runner + nodes).
#
# Cross-platform preflight smoke (no PS1 gates):
#   go run ./cmd/dogego cert weekly-live -skip-scripts -mine-bootstrap -require-wallet-dat
#
# Runbook: docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md (workflow 10)
#
#   .\scripts\ci_live_reboottestnet_gate.ps1
#   .\scripts\ci_live_reboottestnet_gate.ps1 -IncludeCoreAlignedGate -IncludeCorruptionMini
#   $env:DOGEGO_CI_LIVE_GATE = "1"; .\scripts\core_operator_workflow_cert.ps1
param(
    [switch]$IncludeCoreAlignedGate,
    [switch]$IncludeCorruptionMini,
    [switch]$IncludeCorruptionLongSoak,
    [switch]$SkipOffline,
    [string]$DataDir = "dogedata",
    [string]$Network = "reboottestnet",
    [string]$DogeGoRpcPort = "44556"
)
$ErrorActionPreference = "Stop"

if ($Network -ne "reboottestnet") {
    Write-Error "ci_live_reboottestnet_gate is reboottestnet-only."
}

$steps = @()
$failed = $false

function Step {
    param([string]$Name, [string]$Script, [hashtable]$Args = @{})
    Write-Host "`n=== $Name ===" -ForegroundColor Cyan
    & $Script @Args
    $ok = ($LASTEXITCODE -eq 0)
    $script:steps += [ordered]@{ name = $Name; ok = $ok; exit = $LASTEXITCODE }
    if (-not $ok) { $script:failed = $true }
    return $ok
}

Write-Host "=== CI live reboottestnet gate ===" -ForegroundColor Cyan

$env:DOGEGO_RPC_PORT = $DogeGoRpcPort

& "$PSScriptRoot\ci_runner_preflight.ps1" -RequireCore -DogeGoRpcPort $DogeGoRpcPort
if ($LASTEXITCODE -ne 0) { exit 2 }

if (-not $SkipOffline -and $env:DOGEGO_CI_SKIP_OFFLINE -ne "1") {
    $DogeGo = Split-Path -Parent $PSScriptRoot
    Set-Location $DogeGo
    go test ./docs/... ./ui/... -count=1
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    go test ./consensus -run 'TestStatefulMempool' -count=1
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

$common = @{ DataDir = $DataDir; Network = $Network }

Step "node_health" "$PSScriptRoot\node_health.ps1" $common
if ($failed) { exit 2 }

Step "e2e_reboottestnet" "$PSScriptRoot\core_e2e_reboottestnet_runbook.ps1" @{
    Network       = $Network
    DataDir       = $DataDir
    DogeGoRpcPort = $DogeGoRpcPort
    IncludeCoreCompare = $true
}

if ($IncludeCoreAlignedGate -or $env:DOGEGO_REBOOTTESTNET_CORE_GATE -eq "1") {
    $env:DOGEGO_CORE_COMPARE_MIN = "24"
    Step "core_aligned_22" "$PSScriptRoot\core_reboottestnet_core_aligned_gate.ps1" @{
        DogeGoRpcPort = $DogeGoRpcPort
    }
}

if ($IncludeCorruptionMini -or $env:DOGEGO_CORRUPTION_EXTENDED_MINI -eq "1") {
    Step "corruption_extended_mini" "$PSScriptRoot\corruption_extended_cert_mini.ps1" @{
        Network = $Network
        DataDir = $DataDir
    }
}

if ($IncludeCorruptionLongSoak -or $env:DOGEGO_CORRUPTION_LONG_SOAK -eq "1") {
    Step "corruption_long_soak" "$PSScriptRoot\ci_scheduled_corruption_soak.ps1" @{
        Network = $Network
        DataDir = $DataDir
    }
}

if ($failed) {
    Write-Host "`nCI live reboottestnet gate failed." -ForegroundColor Red
    foreach ($s in $steps) {
        if (-not $s.ok) { Write-Host ("  FAIL: " + $s.name) -ForegroundColor Red }
    }
    exit 1
}

Write-Host "`nCI live reboottestnet gate passed." -ForegroundColor Green
exit 0
