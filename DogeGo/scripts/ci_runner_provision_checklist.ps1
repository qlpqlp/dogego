# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: one-shot operator checklist for dogego-live CI runner provisioning.
#
# Cross-platform: dogego cert provision
#   go run ./cmd/dogego cert provision -preflight -json
#   go run ./cmd/dogego cert provision -preflight -run-setup -mine-bootstrap
#
# Runbook: docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md (workflow 10)
#
#   .\scripts\ci_runner_provision_checklist.ps1
param(
    [switch]$Json,
    [switch]$RunPreflight,
    [switch]$RunSetup
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\_wallet_dat_env.ps1"

$checklist = @(
    [ordered]@{ step = 1; item = "Install Go 1.22+ and add to PATH"; done = $false }
    [ordered]@{ step = 2; item = "Install dogecoin-cli (Core) and set DOGEGO_CORE_CLI if non-default"; done = $false }
    [ordered]@{ step = 3; item = "Register GitHub self-hosted runner with label dogego-live"; done = $false }
    [ordered]@{ step = 4; item = "Run DogeGo node on reboottestnet RPC :44556 with wallet enabled"; done = $false }
    [ordered]@{ step = 5; item = "Run Core on reboottestnet RPC :44555 with wallet enabled"; done = $false }
    [ordered]@{ step = 6; item = "Set DOGEGO_SCHEDULED_WEEKLY_LIVE=1 and DOGEGO_SCHEDULED_LIVE_SOAK=1 (gh_enable_scheduled_live.ps1)"; done = $false }
    [ordered]@{ step = 7; item = "Run dogego cert setup-parity -mine-bootstrap (or setup_reboottestnet_core_parity.ps1 -MineBootstrap)"; done = $false }
    [ordered]@{ step = 8; item = "Dispatch DogeGo workflow with live_soak=true (Milestone B full) or live_weekly=true"; done = $false }
    [ordered]@{ step = 9; item = "Optional: provision Core wallet.dat (provision_wallet_dat_fixture.ps1 -SetUserEnv)"; done = $false }
)

$auto = @()
if (Get-Command go -ErrorAction SilentlyContinue) {
    $checklist[0].done = $true
    $auto += "go_ok"
}
if ((Get-Command dogecoin-cli -ErrorAction SilentlyContinue) -or $env:DOGEGO_CORE_CLI) {
    $checklist[1].done = $true
    $auto += "core_cli_ok"
}
if ($env:RUNNER_NAME -or $env:GITHUB_RUNNER_LABELS -match "dogego-live") {
    $checklist[2].done = $true
    $auto += "runner_label_ok"
}
if ($env:DOGEGO_WALLET_DAT -and (Test-Path -LiteralPath $env:DOGEGO_WALLET_DAT)) {
    $checklist[8].done = $true
    $auto += "wallet_dat_fixture_ok"
}

if ($RunPreflight) {
    & "$PSScriptRoot\ci_runner_preflight.ps1" -RequireCore
    if ($LASTEXITCODE -eq 0) {
        $checklist[3].done = $true
        $checklist[4].done = $true
        $auto += "preflight_ok"
    }
}

if ($RunSetup) {
    & "$PSScriptRoot\setup_reboottestnet_core_parity.ps1" -MineBootstrap
    if ($LASTEXITCODE -eq 0) {
        $checklist[6].done = $true
        $auto += "setup_ok"
    }
}

$doneCount = @($checklist | Where-Object { $_.done }).Count
$ok = ($doneCount -ge 4)

if ($Json) {
    [ordered]@{
        ok        = $ok
        done      = $doneCount
        total     = $checklist.Count
        checklist = $checklist
        auto      = $auto
    } | ConvertTo-Json -Depth 5
} else {
    Write-Host "=== dogego-live runner provision checklist ($doneCount/$($checklist.Count)) ===" -ForegroundColor Cyan
    foreach ($row in $checklist) {
        $mark = if ($row.done) { "[x]" } else { "[ ]" }
        $color = if ($row.done) { "Green" } else { "DarkGray" }
        Write-Host ("  {0} {1}. {2}" -f $mark, $row.step, $row.item) -ForegroundColor $color
    }
}

if (-not $ok -and -not $Json) {
    Write-Host "`nComplete unchecked items before enabling scheduled live CI." -ForegroundColor Yellow
}
exit 0
