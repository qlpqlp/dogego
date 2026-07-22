# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Enable DogeGo scheduled live CI repo variables via GitHub CLI.
#
# Cross-platform: dogego cert enable-weekly
#   go run ./cmd/dogego cert enable-weekly -require-wallet-dat
#   go run ./cmd/dogego cert weekly-live -mine-bootstrap -require-wallet-dat
#
# Runbook: docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md (workflow 10)
#
#   .\scripts\gh_enable_scheduled_live.ps1
#   .\scripts\gh_enable_scheduled_live.ps1 -WeeklyOnly
param(
    [switch]$WeeklyOnly,
    [switch]$RequireWalletDat,
    [switch]$DryRun,
    [string]$Repo = ""
)
$ErrorActionPreference = "Stop"

$gh = Get-Command gh -ErrorAction SilentlyContinue
if (-not $gh) {
    Write-Error "gh CLI not found. Install GitHub CLI and authenticate: gh auth login"
}

if (-not $Repo) {
    if ($env:GITHUB_REPOSITORY) {
        $Repo = $env:GITHUB_REPOSITORY
    } else {
        $remote = (git -C (Split-Path -Parent $PSScriptRoot) remote get-url origin 2>$null)
        if ($remote -match 'github\.com[:/](.+?)(?:\.git)?$') {
            $Repo = $Matches[1]
        }
    }
}
if (-not $Repo) {
    Write-Error "Could not detect repo. Pass -Repo owner/name"
}

$vars = @(
    @{ Name = "DOGEGO_SCHEDULED_WEEKLY_LIVE"; Value = "1"; Note = "weekly: Core 24/24 + corruption mini" }
)
if (-not $WeeklyOnly) {
    $vars += @(
        @{ Name = "DOGEGO_SCHEDULED_CORE_GATE"; Value = "1"; Note = "weekly: Core-aligned gate only" }
        @{ Name = "DOGEGO_SCHEDULED_LIVE_SOAK"; Value = "1"; Note = "weekly: corruption long soak" }
    )
}
if ($RequireWalletDat) {
    $vars += @{ Name = "DOGEGO_WALLET_DAT_REQUIRED"; Value = "1"; Note = "weekly: require live Core wallet.dat probe" }
}

Write-Host "=== Enable DogeGo scheduled live CI ($Repo) ===" -ForegroundColor Cyan
foreach ($v in $vars) {
    $cmd = "gh variable set $($v.Name) --body `"$($v.Value)`" --repo $Repo"
    Write-Host ("  {0} - {1}" -f $v.Name, $v.Note) -ForegroundColor DarkGray
    if ($DryRun) {
        Write-Host "    DRY: $cmd" -ForegroundColor DarkYellow
    } else {
        & gh variable set $v.Name --body $v.Value --repo $Repo
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
}

Write-Host "`nDone. Ensure self-hosted runner has label dogego-live." -ForegroundColor Green
$dispatch = "gh workflow run dogego.yml --repo $Repo -f live_weekly=true"
if ($RequireWalletDat) { $dispatch += " -f require_wallet_dat=true" }
Write-Host "Dispatch test: $dispatch" -ForegroundColor DarkGray
exit 0
