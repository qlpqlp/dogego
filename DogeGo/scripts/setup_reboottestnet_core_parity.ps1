# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone D: prepare reboottestnet DogeGo + Core for stateful mempool Core parity (24/24).
#
# Cross-platform: dogego cert setup-parity
#   go run ./cmd/dogego cert setup-parity
#   go run ./cmd/dogego cert setup-parity -mine-bootstrap
#
# Runbook: docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md (workflow 10)
#
#   .\scripts\setup_reboottestnet_core_parity.ps1
#   .\scripts\setup_reboottestnet_core_parity.ps1 -MineBootstrap
param(
    [switch]$Json,
    [switch]$MineBootstrap,
    [int]$MineBlocks = 101,
    [string]$DogeGoRpcPort = "44556",
    [string]$CoreRpcPort = "44555"
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

Write-Host "=== Reboottestnet Core parity setup ===" -ForegroundColor Cyan

& "$PSScriptRoot\ci_runner_preflight.ps1" -RequireCore -DogeGoRpcPort $DogeGoRpcPort -CoreRpcPort $CoreRpcPort
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$rpcPort = [int]$DogeGoRpcPort
$notes = @()

function Invoke-Dg {
    param([string]$Method, [object[]]$Params = @())
    return Invoke-DogeGoJsonRpc -Method $Method -Params $Params -RpcPort $rpcPort -WarmupRetries 5 -WarmupDelaySec 2
}

$wallet = Invoke-Dg getwalletinfo
$balance = [double]$wallet.balance
$notes += "dogego_balance=$balance"

if ($MineBootstrap -and $balance -lt 1.0) {
    Write-Host "Mining $MineBlocks blocks for DogeGo wallet bootstrap ..." -ForegroundColor Yellow
    $addr = Invoke-Dg getnewaddress -Params @("core_parity_bootstrap")
    $hashes = Invoke-Dg generatetoaddress -Params @($MineBlocks, $addr)
    if (-not $hashes -or @($hashes).Count -lt 1) {
        Write-Error "generatetoaddress returned no blocks"
    }
    $wallet = Invoke-Dg getwalletinfo
    $notes += "dogego_balance_after_mine=$($wallet.balance)"
}

$coreCli = $env:DOGEGO_CORE_CLI
if (-not $coreCli) {
    $coreCli = (Get-Command dogecoin-cli -ErrorAction SilentlyContinue).Source
}
if ($coreCli) {
    $cw = & $coreCli @("-rpcport=$CoreRpcPort", "getwalletinfo") 2>&1
    if ($LASTEXITCODE -eq 0) {
        $coreWallet = $cw | ConvertFrom-Json
        $notes += "core_balance=$($coreWallet.balance)"
    } else {
        Write-Host "WARN: Core wallet not available - stateful Core compare may fail on wallet-anchored rows" -ForegroundColor Yellow
        $notes += "core_wallet_unavailable"
    }
}

$env:DOGEGO_CORE_COMPARE = "1"
$env:DOGEGO_CORE_COMPARE_REQUIRED = "1"
$env:DOGEGO_CORE_COMPARE_MIN = "24"
$env:DOGEGO_CORE_RPC_PORT = $CoreRpcPort
if ($coreCli) { $env:DOGEGO_CORE_CLI = $coreCli }

Write-Host "`nSetup checks passed. Run stateful Core gate:" -ForegroundColor Green
Write-Host "  .\scripts\core_reboottestnet_core_aligned_gate.ps1 -DogeGoRpcPort $DogeGoRpcPort" -ForegroundColor DarkGray
Write-Host "Enable scheduled CI: set repo vars DOGEGO_SCHEDULED_CORE_GATE=1 and DOGEGO_SCHEDULED_LIVE_SOAK=1" -ForegroundColor DarkGray

if ($Json) {
    [ordered]@{ ok = $true; notes = $notes } | ConvertTo-Json -Depth 3
}

exit 0
