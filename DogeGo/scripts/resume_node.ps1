# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Start dogego node if JSON-RPC is not already responding.
# Usage:
#   .\scripts\resume_node.ps1
#   .\scripts\resume_node.ps1 -WaitSec 15
param(
    [int]$WaitSec = 10
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"
$root = Get-DogeGoRepoRoot
$bin = Join-Path $root "dogego.exe"
if (-not (Test-Path $bin)) {
    Write-Host "Build first: go build -o dogego.exe ./cmd/dogego" -ForegroundColor Red
    exit 2
}
if (Get-Process dogego -ErrorAction SilentlyContinue) {
    Write-Host 'dogego process already running - use .\scripts\restart_node.ps1 to replace it' -ForegroundColor Yellow
    exit 1
}
if (Remove-DogeGoStaleProcessLock) {
    Write-Host 'Removed stale process lock from crashed node' -ForegroundColor Yellow
}
try {
    $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 5 -WarmupDelaySec 2
    Write-Host ("Node already running: headers={0} blocks={1} rpc=ready" -f $info.headers, $info.blocks) -ForegroundColor Green
    exit 0
} catch {
    if ($_.Exception.Message -match "warming up|-28") {
        Write-Host "Node RPC port up but still warming up; restart with a fresh build if this persists >2 min." -ForegroundColor Yellow
    }
    Write-Host "Starting dogego node..." -ForegroundColor Cyan
}
Start-Process -FilePath $bin -WorkingDirectory $root -ArgumentList "node","-datadir","dogedata","-network","mainnet" -WindowStyle Normal
if ($WaitSec -gt 0) {
    Start-Sleep -Seconds $WaitSec
    try {
        $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 8 -WarmupDelaySec 2
        Write-Host ("RPC up: headers={0} blocks={1} ibd={2}" -f $info.headers, $info.blocks, $info.initialblockdownload) -ForegroundColor Green
        exit 0
    } catch {
        Write-Host "Node started; RPC not ready yet. Try .\scripts\sync_status.ps1 in a few seconds." -ForegroundColor Yellow
        exit 0
    }
}
