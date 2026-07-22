# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone B/D/E: self-hosted dogego-live runner preflight (tools + live RPC readiness).
#
# Cross-platform: dogego cert preflight
#   go run ./cmd/dogego cert preflight -require-core -require-wallet-dat
#   go run ./cmd/dogego cert preflight -offline
#
# Runbook: docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md (workflow 10)
#
#   .\scripts\ci_runner_preflight.ps1
#   .\scripts\ci_runner_preflight.ps1 -RequireCore -Json
#   .\scripts\ci_runner_preflight.ps1 -OfflineOnly
param(
    [switch]$Json,
    [switch]$RequireCore,
    [switch]$RequireWalletDat,
    [switch]$OfflineOnly,
    [string]$DogeGoRpcPort = "44556",
    [string]$CoreRpcPort = "44555",
    [int]$MaxBlockDelta = 3
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"
. "$PSScriptRoot\_wallet_dat_env.ps1"

$issues = @()
$warnings = @()
$notes = @()

function Test-PortListen {
    param(
        [int]$Port,
        [string]$HostName = "127.0.0.1",
        [int]$TimeoutMs = 500
    )
    if (Get-Command Get-NetTCPConnection -ErrorAction SilentlyContinue) {
        try {
            $c = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
            if ($null -ne $c) { return $true }
        } catch { }
    }
    try {
        $client = [System.Net.Sockets.TcpClient]::new()
        $iar = $client.BeginConnect($HostName, $Port, $null, $null)
        $ok = $iar.AsyncWaitHandle.WaitOne($TimeoutMs, $false)
        if ($ok -and $client.Connected) {
            $client.Close()
            return $true
        }
        $client.Close()
        return $false
    } catch {
        return $false
    }
}

function Resolve-CoreCli {
    $coreCli = $env:DOGEGO_CORE_CLI
    if (-not $coreCli) {
        $coreCli = (Get-Command dogecoin-cli -ErrorAction SilentlyContinue).Source
    }
    if (-not $coreCli) {
        $d = "C:\Program Files\Dogecoin\daemon\dogecoin-cli.exe"
        if (Test-Path $d) { $coreCli = $d }
    }
    return $coreCli
}

Write-Host "=== CI runner preflight (dogego-live) ===" -ForegroundColor Cyan

$goVer = $null
try {
    $goVer = (go version 2>&1 | Out-String).Trim()
    if (-not $goVer) { $issues += "go_missing" }
    else { $notes += $goVer }
} catch {
    $issues += "go_missing"
}

if ($OfflineOnly) {
    $ok = ($issues.Count -eq 0)
    $report = [ordered]@{ ok = $ok; offline_only = $true; issues = @($issues); notes = @($notes) }
    if ($Json) { $report | ConvertTo-Json -Depth 4 }
    elseif ($ok) { Write-Host "Offline runner preflight passed." -ForegroundColor Green }
    else { Write-Host "Offline runner preflight failed." -ForegroundColor Red }
    if (-not $ok) { exit 1 }
    exit 0
}

if (-not (Test-PortListen ([int]$DogeGoRpcPort))) {
    $issues += "dogego_rpc_port_not_listening"
} else {
    $notes += "dogego_port_listening=$DogeGoRpcPort"
}

$coreCli = Resolve-CoreCli
if ($RequireCore -or $env:DOGEGO_CORE_COMPARE_REQUIRED -eq "1") {
    if (-not $coreCli) {
        $issues += "dogecoin_cli_missing"
    } elseif (-not (Test-PortListen ([int]$CoreRpcPort))) {
        $issues += "core_rpc_port_not_listening"
    } else {
        $notes += "core_port_listening=$CoreRpcPort"
    }
}

try {
    $dg = Invoke-DogeGoJsonRpc -Method getblockchaininfo -RpcPort ([int]$DogeGoRpcPort) -WarmupRetries 5 -WarmupDelaySec 2
    $notes += "dogego_chain=$($dg.chain) blocks=$($dg.blocks)"
    if ($dg.chain -and "$($dg.chain)" -notmatch "test|reboot") {
        $warnings += "dogego_not_testnet_chain"
    }
} catch {
    $issues += "dogego_rpc_unreachable"
}

try {
    $dgWallet = Invoke-DogeGoJsonRpc -Method getwalletinfo -RpcPort ([int]$DogeGoRpcPort) -WarmupRetries 3
    if (-not $dgWallet) { $issues += "dogego_wallet_disabled" }
} catch {
    $issues += "dogego_wallet_unreachable"
}

$coreInfo = $null
if ($coreCli -and (Test-PortListen ([int]$CoreRpcPort))) {
    $coreArgs = @("-rpcport=$CoreRpcPort", "getblockchaininfo")
    if ($env:DOGEGO_CORE_RPC_USER) {
        $coreArgs = @("-rpcuser=$env:DOGEGO_CORE_RPC_USER", "-rpcpassword=$env:DOGEGO_CORE_RPC_PASS") + $coreArgs
    }
    $out = & $coreCli @coreArgs 2>&1
    if ($LASTEXITCODE -eq 0) {
        $coreInfo = $out | ConvertFrom-Json
        $notes += "core_chain=$($coreInfo.chain) blocks=$($coreInfo.blocks)"
    } elseif ($RequireCore) {
        $issues += "core_rpc_unreachable"
    } else {
        $warnings += "core_rpc_unreachable"
    }
}

if ($dg -and $coreInfo) {
    $delta = [Math]::Abs([int64]$dg.blocks - [int64]$coreInfo.blocks)
    $notes += "block_delta=$delta"
    if ($delta -gt $MaxBlockDelta) {
        if ($RequireCore) { $issues += "block_height_delta_too_large" }
        else { $warnings += "block_height_delta_too_large" }
    }
}

$discoveredWalletDat = Initialize-WalletDatEnv
if ($discoveredWalletDat) {
    $notes += "wallet_dat_auto_discovered=$discoveredWalletDat"
}

if ($env:DOGEGO_WALLET_DAT) {
    $walletArgs = @("cert", "wallet-migration", "-skip-offline", "-live-probe", "-network", "reboottestnet", "-json", "-wallet-dat", $env:DOGEGO_WALLET_DAT)
    if ($env:DOGEGO_WALLET_DAT_PASSPHRASE) {
        $walletArgs += @("-passphrase", $env:DOGEGO_WALLET_DAT_PASSPHRASE)
    }
    if ($RequireWalletDat -or $env:DOGEGO_WALLET_DAT_REQUIRED -eq "1") {
        $walletArgs += "-require-wallet-dat"
    }
    $walletOut = & go run ./cmd/dogego @walletArgs 2>&1
    if ($LASTEXITCODE -eq 0) {
        $notes += "wallet_dat_probe_ok"
    } else {
        if ($RequireWalletDat -or $env:DOGEGO_WALLET_DAT_REQUIRED -eq "1") { $issues += "wallet_dat_probe_failed" }
        else { $warnings += "wallet_dat_probe_failed" }
        $walletErr = ($walletOut | Out-String).Trim()
        $notes += "wallet_dat_error=$walletErr"
    }
} elseif ($RequireWalletDat -or $env:DOGEGO_WALLET_DAT_REQUIRED -eq "1") {
    $issues += "wallet_dat_required_missing"
}

if ($env:RUNNER_NAME) { $notes += "runner=$($env:RUNNER_NAME)" }
if ($env:RUNNER_LABELS) { $notes += "labels=$($env:RUNNER_LABELS)" }
elseif ($env:GITHUB_RUNNER_LABELS) { $notes += "labels=$($env:GITHUB_RUNNER_LABELS)" }

$ok = ($issues.Count -eq 0)
$report = [ordered]@{
    ok       = $ok
    issues   = @($issues)
    warnings = @($warnings)
    notes    = @($notes)
    dogego   = if ($dg) { @{ blocks = $dg.blocks; chain = $dg.chain } } else { $null }
    core     = if ($coreInfo) { @{ blocks = $coreInfo.blocks; chain = $coreInfo.chain } } else { $null }
}

if ($Json) {
    $report | ConvertTo-Json -Depth 5
} else {
    foreach ($n in $notes) { Write-Host ("NOTE: " + $n) -ForegroundColor DarkGray }
    foreach ($w in $warnings) { Write-Host ("WARN: " + $w) -ForegroundColor Yellow }
    foreach ($i in $issues) { Write-Host ("FAIL: " + $i) -ForegroundColor Red }
    if ($ok) {
        Write-Host "`nCI runner preflight passed." -ForegroundColor Green
    } else {
        Write-Host "`nCI runner preflight failed." -ForegroundColor Red
        Write-Host "Hint: register self-hosted runner with label 'dogego-live'; start DogeGo reboottestnet on :$DogeGoRpcPort" -ForegroundColor DarkGray
        if ($RequireCore) {
            Write-Host "Hint: start Core reboottestnet on :$CoreRpcPort with wallet enabled (set DOGEGO_CORE_CLI if needed)" -ForegroundColor DarkGray
        }
    }
}

if (-not $ok) { exit 1 }
exit 0
