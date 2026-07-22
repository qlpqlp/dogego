# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Side-by-side Core vs DogeGo when Dogecoin Core already owns RPC :22555.
# Starts DogeGo on :22557 (if not listening), runs parity probes, leaves DogeGo running.
#
#   .\scripts\core_compare_with_core.ps1
#   .\scripts\core_compare_with_core.ps1 -SkipStart   # DogeGo already on :22557
param(
    [string]$DataDir = "dogedata",
    [string]$Network = "mainnet",
    [string]$DogeGoRpcPort = "22557",
    [string]$CoreRpcPort = "22555",
    [int]$WaitSec = 45,
    [switch]$SkipStart,
    [switch]$MempoolProbe
)
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

$env:DOGEGO_RPC_PORT = $DogeGoRpcPort
$env:DOGEGO_CORE_RPC_PORT = $CoreRpcPort
$env:DOGEGO_CORE_COMPARE = "1"
if (-not $env:DOGEGO_CORE_CLI) {
    $coreDefault = "C:\Program Files\Dogecoin\daemon\dogecoin-cli.exe"
    if (Test-Path $coreDefault) { $env:DOGEGO_CORE_CLI = $coreDefault }
}
if (-not $env:DOGEGO_PARITY_MAX_HEADER_DELTA) { $env:DOGEGO_PARITY_MAX_HEADER_DELTA = "600000" }
if (-not $env:DOGEGO_PARITY_MAX_BLOCK_DELTA) { $env:DOGEGO_PARITY_MAX_BLOCK_DELTA = "6250000" }

. "$PSScriptRoot\dogego_rpc.ps1"

function Test-PortListen([int]$Port) {
    $c = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
    return ($null -ne $c)
}

if (-not $SkipStart) {
    if (Test-PortListen ([int]$DogeGoRpcPort)) {
        Write-Host "DogeGo RPC already listening on port $DogeGoRpcPort" -ForegroundColor DarkGray
    } else {
        if (Test-PortListen 22555) {
            Write-Host "Core on port 22555 - starting DogeGo on port $DogeGoRpcPort" -ForegroundColor Cyan
        }
        $bin = Join-Path $DogeGo "dogego.exe"
        if (-not (Test-Path $bin)) {
            Write-Host "Building dogego.exe..." -ForegroundColor Cyan
            go build -o dogego.exe ./cmd/dogego
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        }
        $rpcAddr = "127.0.0.1:$DogeGoRpcPort"
        Write-Host "Starting dogego node rpc=$rpcAddr network=$Network" -ForegroundColor Cyan
        Start-Process -FilePath $bin -WorkingDirectory $DogeGo -ArgumentList @(
            "node", "-datadir", $DataDir, "-network", $Network, "-rpc", $rpcAddr
        ) -WindowStyle Normal
        $deadline = (Get-Date).AddSeconds($WaitSec)
        $ready = $false
        while ((Get-Date) -lt $deadline) {
            Start-Sleep -Seconds 2
            try {
                $null = Invoke-DogeGoJsonRpc -Method getblockchaininfo -RpcPort ([int]$DogeGoRpcPort) -WarmupRetries 1
                $ready = $true
                break
            } catch {
                if ($_.Exception.Message -notmatch "warming up|-28") {
                    Write-Host "  waiting: $($_.Exception.Message)" -ForegroundColor DarkGray
                }
            }
        }
        if (-not $ready) {
            Write-Error "DogeGo RPC not ready on port $DogeGoRpcPort within ${WaitSec}s"
        }
        Write-Host "DogeGo RPC ready on port $DogeGoRpcPort" -ForegroundColor Green
    }
}

Write-Host ""
Write-Host "=== Core parity probe (Core port $CoreRpcPort vs DogeGo port $DogeGoRpcPort) ===" -ForegroundColor Cyan
& "$PSScriptRoot\core_parity_probe.ps1"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if ($MempoolProbe) {
    Write-Host ""
    Write-Host "=== Mempool parity probe ===" -ForegroundColor Cyan
    & "$PSScriptRoot\core_mempool_parity_probe.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Write-Host ""
Write-Host "Core compare passed (DogeGo left running on port $DogeGoRpcPort)." -ForegroundColor Green
exit 0
