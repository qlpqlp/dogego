# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Persist utxo.cache from the running node (async; polls until disk matches chainActive).
# Usage:
#   .\scripts\save_utxo_snapshot.ps1
param(
    [string]$DataDir,
    [string]$Network = "mainnet",
    [int]$WaitSec = 120
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

$info = $null
try { $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 2 } catch { }
if ($info -and $info.dogego_utxo_bodies_aligned -eq $false) {
    Write-Host "WARN: bodies lag UTXO - saveutxosnapshot refused until replay catches up" -ForegroundColor Yellow
    exit 1
}

$res = Invoke-DogeGoJsonRpc -Method saveutxosnapshot -WarmupRetries 3
$target = [int64]$res.height
Write-Host ("saveutxosnapshot: queued height={0} outputs={1}" -f $target, $res.outputs) -ForegroundColor Cyan
Write-Host ("path: {0}" -f $res.path)
if ($res.already_in_flight) {
    Write-Host "snapshot save already in flight" -ForegroundColor Yellow
}

$chainDir = (Get-DogeGoDiskSyncSnapshot -DataDir $DataDir -Network $Network).ChainDir
$cachePath = Join-Path $chainDir "utxo.cache"
$deadline = (Get-Date).AddSeconds($WaitSec)
do {
    Start-Sleep -Seconds 3
    try {
        $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 1
        $disk = $info.dogego_utxo_snapshot_height
        if ($null -ne $disk -and [int64]$disk -ge $target) {
            Write-Host ("utxo.cache on disk through height {0}" -f $disk) -ForegroundColor Green
            if (Test-Path $cachePath) {
                Get-Item $cachePath | Select-Object FullName, LastWriteTime, Length
            }
            exit 0
        }
        if ($info.dogego_utxo_snapshot_save_in_flight -ne $true) {
            $mem = [int64]$info.dogego_utxo_chain_active
            if ($null -ne $disk -and [int64]$disk -ge $mem -and $mem -ge $target) {
                Write-Host ("utxo.cache through {0} (chainActive {1})" -f $disk, $mem) -ForegroundColor Green
                exit 0
            }
        }
    } catch {
        Write-Host "waiting for RPC/snapshot..." -ForegroundColor DarkGray
    }
} while ((Get-Date) -lt $deadline)

Write-Host "WARN: snapshot not confirmed on disk within ${WaitSec}s; check getblockchaininfo dogego_utxo_snapshot_height" -ForegroundColor Yellow
exit 1
