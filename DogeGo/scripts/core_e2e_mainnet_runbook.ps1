# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: mainnet read-only end-to-end operator runbook (non-disruptive).
#
#   .\scripts\core_e2e_mainnet_runbook.ps1 -AllowMainnet
param(
    [switch]$Json,
    [switch]$AllowMainnet,
    [string]$DataDir = "dogedata",
    [string]$Network = "mainnet"
)
$ErrorActionPreference = "Stop"
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

if ($Network -ne "mainnet") {
    Write-Error "This runbook targets mainnet (read-only Core compare)."
}
if (-not $AllowMainnet) {
    Write-Error "Pass -AllowMainnet to run mainnet read-only E2E bundle."
}

$common = @{ DataDir = $DataDir; Network = $Network; AllowMainnet = $true }

Step "offline_mempool_corpus" "$PSScriptRoot\core_mempool_corpus_probe.ps1" @{}
if ($failed) { exit 2 }

Step "mainnet_side_by_side" "$PSScriptRoot\core_mainnet_side_by_side_runbook.ps1" $common
Step "mainnet_maintenance" "$PSScriptRoot\core_mainnet_maintenance_compare.ps1" $common
Step "mainnet_reindex_compare" "$PSScriptRoot\core_mainnet_reindex_compare.ps1" $common

$allOk = -not $failed
if ($Json) {
    [ordered]@{ ok = $allOk; network = $Network; steps = $steps } | ConvertTo-Json -Depth 4
} else {
    if ($allOk) {
        Write-Host "`nMainnet read-only E2E runbook passed." -ForegroundColor Green
    } else {
        Write-Host "`nMainnet read-only E2E runbook failed." -ForegroundColor Red
        foreach ($s in $steps) {
            if (-not $s.ok) { Write-Host ("  FAIL: " + $s.name) -ForegroundColor Red }
        }
    }
}

if (-not $allOk) { exit 1 }
exit 0
