# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone B (partial): run live corruption inject probes for headers, raw, and filter artifacts.
# DISRUPTIVE - reboottestnet default; requires an existing chain datadir with bodies/filters for raw/filter targets.
#
#   .\scripts\corruption_inject_soak.ps1
#   .\scripts\corruption_inject_soak.ps1 -Targets headers,raw -DataDir dogedata
param(
    [string[]]$Targets = @("headers", "raw", "bundled", "filter", "txindex"),
    [string]$DataDir = "dogedata",
    [string]$Network = "reboottestnet",
    [switch]$AllowMainnet,
    [int]$WaitSec = 45
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

Write-Host "=== Live corruption inject soak ($($Targets -join ', ')) ===" -ForegroundColor Cyan
$summary = @()
foreach ($target in $Targets) {
    Write-Host "`n--- target: $target ---" -ForegroundColor Yellow
    $args = @(
        "-Target", $target,
        "-DataDir", $DataDir,
        "-Network", $Network,
        "-WaitSec", $WaitSec
    )
    if ($AllowMainnet) { $args += "-AllowMainnet" }
    try {
        $beforeBlocks = $null
        try {
            $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 1 -WarmupDelaySec 1
            $beforeBlocks = [int64]$info.blocks
        } catch { }
        & "$PSScriptRoot\corruption_inject_live.ps1" @args
        if ($LASTEXITCODE -ne 0) {
            if ($target -eq "bundled") {
                Write-Host "Skipping bundled inject (no blk00000.dat or node recovery failed)" -ForegroundColor DarkGray
                continue
            }
            Write-Error "corruption inject failed for target $target"
        }
        $afterBlocks = $null
        try {
            $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 2 -WarmupDelaySec 2
            $afterBlocks = [int64]$info.blocks
        } catch {
            Write-Error "getblockchaininfo after $target inject: $_"
        }
        if ($null -ne $beforeBlocks -and $afterBlocks -lt ($beforeBlocks - 2)) {
            Write-Error "block height regressed after $target inject: before=$beforeBlocks after=$afterBlocks"
        }
        $summary += [pscustomobject]@{
            Target = $target
            BlocksBefore = $beforeBlocks
            BlocksAfter  = $afterBlocks
        }
    } catch {
        if ($target -eq "bundled") {
            Write-Host "Skipping bundled inject: $_" -ForegroundColor DarkGray
            continue
        }
        throw
    }
}
if ($summary.Count -gt 0) {
    Write-Host "`nConvergence summary:" -ForegroundColor DarkGray
    $summary | Format-Table -AutoSize | Out-String | Write-Host
}
Write-Host "`nCorruption inject soak passed for all targets." -ForegroundColor Green
exit 0
