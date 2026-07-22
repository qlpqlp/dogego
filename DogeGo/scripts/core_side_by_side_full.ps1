# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: full Core vs DogeGo side-by-side probe bundle (both nodes required).
# Skips individual probes when dogecoin-cli is absent.
#
#   .\scripts\core_side_by_side_full.ps1
param(
    [switch]$Json
)
$ErrorActionPreference = "Stop"
$steps = @()
$failed = $false

function Run-Step($Name, $Script) {
    Write-Host "`n=== $Name ===" -ForegroundColor Cyan
    & $Script
    $ok = ($LASTEXITCODE -eq 0)
    $script:steps += [ordered]@{ name = $Name; ok = $ok; exit = $LASTEXITCODE }
    if (-not $ok) { $script:failed = $true }
}

Run-Step "core_parity_probe" "$PSScriptRoot\core_parity_probe.ps1"
Run-Step "core_mempool_parity" "$PSScriptRoot\core_mempool_parity_probe.ps1"
Run-Step "core_maintenance" "$PSScriptRoot\core_maintenance_workflow.ps1"
Run-Step "core_restart_resume" "$PSScriptRoot\core_restart_resume_check.ps1"
Run-Step "core_bip152" "$PSScriptRoot\core_bip152_probe.ps1"
Run-Step "dogego_end_to_end" "$PSScriptRoot\core_end_to_end_workflow.ps1"

$allOk = -not $failed
if ($Json) {
    [ordered]@{ ok = $allOk; steps = $steps } | ConvertTo-Json -Depth 4
} else {
    if ($allOk) {
        Write-Host "`nCore side-by-side full probe passed." -ForegroundColor Green
    } else {
        Write-Host "`nCore side-by-side full probe had failures." -ForegroundColor Red
        foreach ($s in $steps) {
            if (-not $s.ok) { Write-Host ("  FAIL: " + $s.name) -ForegroundColor Red }
        }
    }
}

if (-not $allOk) { exit 1 }
exit 0
