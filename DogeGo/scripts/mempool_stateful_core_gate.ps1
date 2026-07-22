# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone D: stateful mempool parity with required Core side-by-side (reboottestnet).
#
#   .\scripts\mempool_stateful_core_gate.ps1
#   $env:DOGEGO_MEMPOOL_STATEFUL_CORE_GATE = "1"; .\scripts\core_operator_workflow_cert.ps1
param(
    [string]$Network = "reboottestnet",
    [string]$DogeGoRpcPort = "44556"
)
$ErrorActionPreference = "Stop"

$env:DOGEGO_CORE_COMPARE = "1"
$env:DOGEGO_CORE_COMPARE_REQUIRED = "1"

Write-Host "=== Stateful mempool Core gate (24 scenarios, compare required) ===" -ForegroundColor Cyan
& "$PSScriptRoot\mempool_stateful_parity_reboottestnet.ps1" -Scenario all -Network $Network -DogeGoRpcPort $DogeGoRpcPort
exit $LASTEXITCODE
