# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Restore a quarantined utxo.cache.stale.* snapshot when it carries more chainstate than utxo.cache.
# Usage:
#   .\scripts\restore_utxo_snapshot.ps1
#   .\scripts\restore_utxo_snapshot.ps1 -DataDir dogedata -Network mainnet -Restart
param(
    [string]$DataDir = "dogedata",
    [string]$Network = "mainnet",
    [switch]$Restart
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"
$root = Get-DogeGoRepoRoot
$chainDir = Get-DogeGoChainDir -DataDir $DataDir -Network $Network

$procs = Get-Process dogego -ErrorAction SilentlyContinue
if ($procs) {
    Write-Host "Stopping dogego before utxo restore (avoids saveutxosnapshot overwriting restored file)..." -ForegroundColor Yellow
    foreach ($p in $procs) {
        if ($p.MainWindowHandle -ne 0) {
            [void]$p.CloseMainWindow()
        }
    }
    Start-Sleep -Seconds 20
    $procs = Get-Process dogego -ErrorAction SilentlyContinue
    if ($procs) {
        $procs | Stop-Process -Force
        Start-Sleep -Seconds 2
    }
}

$stale = Get-ChildItem (Join-Path $chainDir "utxo.cache.stale*") -ErrorAction SilentlyContinue | Sort-Object Length -Descending
if (-not $stale) {
    Write-Host "No utxo.cache.stale* files in $chainDir" -ForegroundColor Yellow
    exit 1
}
$pick = $stale | Select-Object -First 1
Write-Host ("Promoting {0} ({1:N0} bytes) -> utxo.cache" -f $pick.Name, $pick.Length) -ForegroundColor Cyan
$dst = Join-Path $chainDir "utxo.cache"
$backup = Join-Path $chainDir ("utxo.cache.backup." + [int][double]::Parse((Get-Date -UFormat %s)))
if (Test-Path $dst) {
    Copy-Item $dst $backup -Force
    Write-Host ("Backed up current utxo.cache to {0}" -f (Split-Path $backup -Leaf)) -ForegroundColor DarkGray
}
Copy-Item $pick.FullName $dst -Force
Write-Host "Restored utxo.cache from stale snapshot." -ForegroundColor Green
if ($Restart) {
    & "$PSScriptRoot\restart_node.ps1" -DataDir $DataDir -Network $Network -WaitSec 40
}
