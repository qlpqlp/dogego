# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Stop any running dogego, optionally rebuild, and start one node instance.
# Usage:
#   .\scripts\restart_node.ps1
#   .\scripts\restart_node.ps1 -Rebuild
#   .\scripts\restart_node.ps1 -DataDir dogedata -Network mainnet -WaitSec 20
param(
    [switch]$Rebuild,
    [string]$DataDir = "dogedata",
    [string]$Network = "mainnet",
    [int]$WaitSec = 15
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"
$root = Get-DogeGoRepoRoot
$bin = Join-Path $root "dogego.exe"

$procs = Get-Process dogego -ErrorAction SilentlyContinue
if ($procs) {
    try {
        $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 2 -WarmupDelaySec 2
        $aligned = $true
        if ($info.PSObject.Properties.Name -contains "dogego_utxo_bodies_aligned") {
            $aligned = [bool]$info.dogego_utxo_bodies_aligned
        }
        if ($aligned) {
            $snap = Invoke-DogeGoJsonRpc -Method saveutxosnapshot -WarmupRetries 1 -WarmupDelaySec 1
            if ($snap.height) {
                Write-Host ("Saved utxo snapshot through height {0} before stop" -f $snap.height) -ForegroundColor Green
            }
        } else {
            $cont = if ($info.dogego_contiguous_raw_height) { $info.dogego_contiguous_raw_height } else { "?" }
            $utxo = if ($info.dogego_utxo_chain_active) { $info.dogego_utxo_chain_active } elseif ($info.dogego_utxo_snapshot_height) { $info.dogego_utxo_snapshot_height } else { "?" }
            Write-Host ("saveutxosnapshot skipped: bodies ({0}) lag UTXO ({1}) - checkpoint in rawblocks_sync.json is preserved" -f $cont, $utxo) -ForegroundColor Yellow
        }
    } catch {
        Write-Host "saveutxosnapshot skipped (RPC not ready or node stopping)" -ForegroundColor DarkGray
    }
    Write-Host ("Stopping {0} dogego process(es) - close the dogego console with Ctrl+C first for utxo.cache save..." -f $procs.Count) -ForegroundColor Yellow
    foreach ($p in $procs) {
        if ($p.MainWindowHandle -ne 0) {
            [void]$p.CloseMainWindow()
        }
    }
    Start-Sleep -Seconds 25
    $procs = Get-Process dogego -ErrorAction SilentlyContinue
    if ($procs) {
        Write-Host "Force stopping remaining dogego process(es)..." -ForegroundColor Yellow
        $procs | Stop-Process -Force
        Start-Sleep -Seconds 2
    }
} else {
    if (Remove-DogeGoStaleProcessLock -DataDir $DataDir -Network $Network) {
        Write-Host "Removing stale process lock (pid not running)..." -ForegroundColor Yellow
    }
}

if ($Rebuild -or -not (Test-Path $bin)) {
    Write-Host "Building dogego.exe..." -ForegroundColor Cyan
    Push-Location $root
    go build -o dogego.exe ./cmd/dogego
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    Pop-Location
}

Write-Host "Starting dogego node..." -ForegroundColor Cyan
Start-Process -FilePath $bin -WorkingDirectory $root -ArgumentList "node","-datadir",$DataDir,"-network",$Network -WindowStyle Normal

if ($WaitSec -le 0) { exit 0 }

$deadline = (Get-Date).AddSeconds($WaitSec)
do {
    Start-Sleep -Seconds 2
    try {
        $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 2 -WarmupDelaySec 1
        Write-Host ("RPC ready: headers={0} blocks={1} ibd={2}" -f $info.headers, $info.blocks, $info.initialblockdownload) -ForegroundColor Green
        $chainDir = Get-DogeGoChainDir -DataDir $DataDir -Network $Network
        $lockPath = Join-Path $chainDir ".dogego-process.lock"
        if (Test-Path $lockPath) {
            Write-Host ("process lock: {0}" -f (Get-Content $lockPath -Raw).Trim()) -ForegroundColor Green
        } else {
            Write-Host "WARN: process lock file not created yet" -ForegroundColor Yellow
        }
        exit 0
    } catch {
        if ($_.Exception.Message -match "warming up|-28") {
            Write-Host "RPC warming up..." -ForegroundColor DarkGray
        }
    }
} while ((Get-Date) -lt $deadline)

Write-Host ('Node started; RPC not ready within ' + $WaitSec + 's. Run scripts/node_health.ps1') -ForegroundColor Yellow
exit 0
