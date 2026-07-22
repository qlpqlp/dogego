# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: live BIP152 compact-block soak gate (timed HB + cmpct relay probe window).
#
#   .\scripts\bip152_live_soak_gate.ps1
#   .\scripts\bip152_live_soak_gate.ps1 -DurationMin 20 -RequireRelayActivity
#   $env:DOGEGO_BIP152_LIVE_SOAK = "1"; .\scripts\core_operator_workflow_cert.ps1
param(
    [int]$DurationMin = 15,
    [int]$IntervalSec = 60,
    [int]$RpcPort = 0,
    [switch]$RequireRelayActivity
)
$ErrorActionPreference = "Stop"

Write-Host "=== BIP152 live soak gate (Milestone E) ===" -ForegroundColor Cyan
Write-Host "Repeated core_bip152_probe over a timed window (HB negotiate + dogego_cmpct_* schema)." -ForegroundColor DarkGray

$soakArgs = @{
    DurationMin = $DurationMin
    IntervalSec = $IntervalSec
}
if ($RpcPort -gt 0) { $soakArgs.RpcPort = $RpcPort }
if ($RequireRelayActivity) { $soakArgs.RequireRelayActivity = $true }

& "$PSScriptRoot\bip152_timed_soak.ps1" @soakArgs
exit $LASTEXITCODE
