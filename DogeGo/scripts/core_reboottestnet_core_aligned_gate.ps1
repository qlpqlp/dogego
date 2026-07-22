# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone D: reboottestnet Core + DogeGo readiness gate, then stateful mempool Core compare (24/24).
#
#   .\scripts\core_reboottestnet_core_aligned_gate.ps1
#   $env:DOGEGO_REBOOTTESTNET_CORE_GATE = "1"; .\scripts\core_operator_workflow_cert.ps1
param(
    [string]$DogeGoRpcPort = "44556",
    [string]$CoreRpcPort = "44555"
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

$coreCli = $env:DOGEGO_CORE_CLI
if (-not $coreCli) {
    $coreCli = (Get-Command dogecoin-cli -ErrorAction SilentlyContinue).Source
}
if (-not $coreCli) {
    $d = "C:\Program Files\Dogecoin\daemon\dogecoin-cli.exe"
    if (Test-Path $d) { $coreCli = $d }
}
if (-not $coreCli) {
    Write-Error "dogecoin-cli not found (set DOGEGO_CORE_CLI)."
}

$env:DOGEGO_CORE_CLI = $coreCli
$env:DOGEGO_CORE_RPC_PORT = $CoreRpcPort
$env:DOGEGO_CORE_COMPARE = "1"
$env:DOGEGO_CORE_COMPARE_REQUIRED = "1"

Write-Host "=== Reboottestnet Core-aligned gate ===" -ForegroundColor Cyan

$coreArgs = @("-rpcport=$CoreRpcPort", "getblockchaininfo")
if ($env:DOGEGO_CORE_RPC_USER) {
    $coreArgs = @("-rpcuser=$env:DOGEGO_CORE_RPC_USER", "-rpcpassword=$env:DOGEGO_CORE_RPC_PASS") + $coreArgs
}
$coreInfo = & $coreCli @coreArgs 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Error ("Core RPC unreachable on :$CoreRpcPort - " + ($coreInfo | Out-String))
}

$dgInfo = Invoke-DogeGoJsonRpc -Method getblockchaininfo -RpcPort ([int]$DogeGoRpcPort) -WarmupRetries 5 -WarmupDelaySec 2
if (-not $dgInfo) {
    Write-Error "DogeGo RPC unreachable on :$DogeGoRpcPort"
}

$coreBlocks = [int64]$coreInfo.blocks
$dgBlocks = [int64]$dgInfo.blocks
if ([Math]::Abs($coreBlocks - $dgBlocks) -gt 3) {
    Write-Host ("WARN: block height delta Core=$coreBlocks DogeGo=$dgBlocks") -ForegroundColor Yellow
}

$coreWallet = & $coreCli @("-rpcport=$CoreRpcPort", "getwalletinfo") 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "WARN: Core wallet not available (stateful compare may be partial)" -ForegroundColor Yellow
}

$dgWallet = Invoke-DogeGoJsonRpc -Method getwalletinfo -RpcPort ([int]$DogeGoRpcPort) -WarmupRetries 3
if (-not $dgWallet) {
    Write-Error "DogeGo wallet not enabled"
}

& "$PSScriptRoot\mempool_stateful_core_gate.ps1" -DogeGoRpcPort $DogeGoRpcPort
exit $LASTEXITCODE
