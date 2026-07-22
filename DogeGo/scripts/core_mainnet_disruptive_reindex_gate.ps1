# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: mainnet disruptive reindex operator sign-off (manual only).
# Runs DogeGo reindextx + verifychain + read-only Core index compare. Does NOT prune mainnet.
#
#   .\scripts\core_mainnet_disruptive_reindex_gate.ps1 -AllowMainnet -ConfirmDisruptive
param(
    [switch]$Json,
    [switch]$AllowMainnet,
    [switch]$ConfirmDisruptive,
    [switch]$IncludeBlockFilters,
    [string]$DogeGoRpcPort = "22557",
    [string]$CoreRpcPort = "22555"
)
$ErrorActionPreference = "Stop"

if (-not $AllowMainnet -or -not $ConfirmDisruptive) {
    Write-Error "Mainnet disruptive reindex requires -AllowMainnet -ConfirmDisruptive."
}

Write-Host "=== Mainnet disruptive reindex operator gate ===" -ForegroundColor Cyan
Write-Host "This runs reindextx on DogeGo mainnet only (no Core mutation, no prune)." -ForegroundColor Yellow

$args = @{
    Network            = "mainnet"
    AllowMainnet       = $true
    ConfirmDisruptive  = $true
    IncludeCoreCompare = $true
    DogeGoRpcPort      = $DogeGoRpcPort
    CoreRpcPort        = $CoreRpcPort
}
if ($IncludeBlockFilters) { $args.IncludeBlockFilters = $true }

& "$PSScriptRoot\core_reindex_prune_disruptive_workflow.ps1" @args
exit $LASTEXITCODE
