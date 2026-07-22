# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone B (extended): timed IBD health window + live corruption inject soak (reboottestnet default).
#
#   .\scripts\extended_operator_soak.ps1
#   .\scripts\extended_operator_soak.ps1 -DurationMin 30 -CorruptionTargets headers,raw,filter
#   .\scripts\extended_operator_soak.ps1 -CorruptionCycles 3 -TimedCorruptionLoop -LoopDurationMin 45
param(
    [int]$DurationMin = 20,
    [int]$IntervalSec = 120,
    [int]$CorruptionCycles = 1,
    [switch]$TimedCorruptionLoop,
    [int]$LoopDurationMin = 30,
    [int]$LoopIntervalMin = 10,
    [string[]]$CorruptionTargets = @("headers", "raw", "filter", "txindex"),
    [string]$DataDir = "dogedata",
    [string]$Network = "reboottestnet",
    [switch]$AllowMainnet,
    [switch]$SkipCorruption
)
$ErrorActionPreference = "Stop"

Write-Host "=== Extended operator soak (timed + corruption) ===" -ForegroundColor Cyan

$soakArgs = @{
    DurationMin  = $DurationMin
    IntervalSec  = $IntervalSec
    DataDir      = $DataDir
    Network      = $Network
}
& "$PSScriptRoot\ibd_timed_soak.ps1" @soakArgs
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if (-not $SkipCorruption) {
    if ($TimedCorruptionLoop) {
        $loopArgs = @{
            DurationMin       = $LoopDurationMin
            IntervalMin       = $LoopIntervalMin
            CorruptionCycles  = $CorruptionCycles
            Targets           = $CorruptionTargets
            DataDir           = $DataDir
            Network           = $Network
            WaitSec           = 60
        }
        if ($AllowMainnet) { $loopArgs.AllowMainnet = $true }
        & "$PSScriptRoot\corruption_timed_loop.ps1" @loopArgs
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } else {
        if ($CorruptionCycles -lt 1) { throw "CorruptionCycles must be >= 1" }
        for ($cycle = 1; $cycle -le $CorruptionCycles; $cycle++) {
            Write-Host "`n=== Corruption inject cycle $cycle / $CorruptionCycles ===" -ForegroundColor Cyan
            $injectArgs = @{
                Targets  = $CorruptionTargets
                DataDir  = $DataDir
                Network  = $Network
                WaitSec  = 60
            }
            if ($AllowMainnet) { $injectArgs.AllowMainnet = $true }
            & "$PSScriptRoot\corruption_inject_soak.ps1" @injectArgs
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
            try {
                . "$PSScriptRoot\dogego_rpc.ps1"
                $vc = Invoke-DogeGoJsonRpc -Method verifychain -Params @(2, 0) -WarmupRetries 2 -WarmupDelaySec 2
                if ($null -eq $vc -or $vc -eq $false) {
                    Write-Error "verifychain 2 0 failed after corruption cycle $cycle"
                }
                Write-Host "verifychain 2 0 ok (cycle $cycle)" -ForegroundColor DarkGray
            } catch {
                Write-Error "verifychain after corruption cycle $cycle`: $_"
            }
        }
    }
}

Write-Host "`nExtended operator soak passed." -ForegroundColor Green
exit 0
