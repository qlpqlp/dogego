# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone B (partial): live datadir corruption injection + restart convergence probe.
# DISRUPTIVE - corrupts chain artifacts on a stopped node, restarts, and asserts RPC recovery.
# Default network is reboottestnet only (pass -AllowMainnet to override).
#
#   .\scripts\corruption_inject_live.ps1
#   .\scripts\corruption_inject_live.ps1 -Target bundled
#   .\scripts\corruption_inject_live.ps1 -Target filter -DataDir dogedata -Network reboottestnet
#   .\scripts\corruption_inject_live.ps1 -Target txindex
param(
    [ValidateSet("headers", "raw", "bundled", "filter", "txindex")]
    [string]$Target = "headers",
    [string]$DataDir = "dogedata",
    [string]$Network = "reboottestnet",
    [switch]$AllowMainnet,
    [int]$WaitSec = 45
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

if ($Network -eq "mainnet" -and -not $AllowMainnet) {
    Write-Error "Refusing mainnet corruption inject. Use -AllowMainnet if intentional."
}

$chainDir = Get-DogeGoChainDir -DataDir $DataDir -Network $Network
Write-Host "=== Live corruption inject ($Target tail truncate) ===" -ForegroundColor Cyan
Write-Host "Chain dir: $chainDir" -ForegroundColor DarkGray

$beforeInfo = $null
try {
    $beforeInfo = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 3 -WarmupDelaySec 2
    Write-Host ("RPC before stop: headers={0} blocks={1}" -f $beforeInfo.headers, $beforeInfo.blocks) -ForegroundColor DarkGray
} catch {
    Write-Host "Node not running (ok for inject)" -ForegroundColor DarkGray
}

& "$PSScriptRoot\restart_node.ps1" -DataDir $DataDir -Network $Network -WaitSec 0 | Out-Null
Start-Sleep -Seconds 3

function Invoke-TailTruncate {
    param([string]$Path, [int]$TruncateBy, [int]$MinSize = 120)
    if (-not (Test-Path $Path)) {
        Write-Error "artifact not found: $Path"
    }
    $bytes = [System.IO.File]::ReadAllBytes($Path)
    if ($bytes.Length -lt $MinSize) {
        Write-Error "$Path too small ($($bytes.Length) bytes) for tail corruption"
    }
    $newLen = $bytes.Length - $TruncateBy
    Write-Host "Truncating $Path by $TruncateBy bytes ($($bytes.Length) -> $newLen)" -ForegroundColor Yellow
    [System.IO.File]::WriteAllBytes($Path, $bytes[0..($newLen - 1)])
}

switch ($Target) {
    "headers" {
        $segDir = Join-Path $chainDir "headers\seg"
        $manifestPath = Join-Path $chainDir "headers\manifest.json"
        if ((Test-Path $segDir) -and (Test-Path $manifestPath)) {
            $man = Get-Content $manifestPath -Raw | ConvertFrom-Json
            $segSize = 2000
            if ($man.segment_size -gt 0) { $segSize = [int]$man.segment_size }
            $tip = [int64]$man.tip_height
            if ($tip -lt 0) { Write-Error "header segment manifest has no tip" }
            $segStart = [math]::Floor($tip / $segSize) * $segSize
            $segFile = Join-Path $segDir ("{0:D10}.bin" -f $segStart)
            if (-not (Test-Path $segFile)) {
                Write-Error "header segment file not found: $segFile"
            }
            Invoke-TailTruncate -Path $segFile -TruncateBy 37 -MinSize 80
        } elseif (Test-Path (Join-Path $chainDir "headers.bin")) {
            Invoke-TailTruncate -Path (Join-Path $chainDir "headers.bin") -TruncateBy 37
        } else {
            Write-Error "no headers.bin or headers/seg layout found under $chainDir"
        }
    }
    "raw" {
        $rawDir = Join-Path $chainDir "rawblocks"
        if (-not (Test-Path $rawDir)) {
            Write-Error "rawblocks directory not found at $rawDir"
        }
        $candidates = Get-ChildItem -Path $rawDir -Filter "*.bin" -File |
            Where-Object { $_.Name -notlike "*.tmp" } |
            Sort-Object Length -Descending
        if (-not $candidates -or $candidates.Count -eq 0) {
            Write-Error "no rawblocks/*.bin files to corrupt"
        }
        $pick = $candidates[0]
        Invoke-TailTruncate -Path $pick.FullName -TruncateBy 50 -MinSize 200
    }
    "bundled" {
        $rawDir = Join-Path $chainDir "rawblocks"
        $bundled = Join-Path $rawDir "blk00000.dat"
        if (-not (Test-Path $bundled)) {
            Write-Error "bundled blk00000.dat not found at $bundled (enable bundled block layout and sync blocks first)"
        }
        Invoke-TailTruncate -Path $bundled -TruncateBy 50 -MinSize 200
    }
    "filter" {
        $filterDir = Join-Path $chainDir "filters\basic"
        if (-not (Test-Path $filterDir)) {
            Write-Error "filters/basic directory not found at $filterDir"
        }
        $candidates = Get-ChildItem -Path $filterDir -Filter "*.dat" -File |
            Where-Object { $_.Name -notlike "*.tmp" } |
            Sort-Object Length -Descending
        if (-not $candidates -or $candidates.Count -eq 0) {
            Write-Error "no filters/basic/*.dat files to corrupt"
        }
        $pick = $candidates[0]
        Invoke-TailTruncate -Path $pick.FullName -TruncateBy 20 -MinSize 40
    }
    "txindex" {
        $txDir = Join-Path $chainDir "indexes\tx"
        if (-not (Test-Path $txDir)) {
            Write-Error "indexes/tx directory not found at $txDir (enable tx index and sync blocks first)"
        }
        $candidates = Get-ChildItem -Path $txDir -File |
            Where-Object { $_.Name -notlike "*.tmp" } |
            Sort-Object Length -Descending
        if (-not $candidates -or $candidates.Count -eq 0) {
            Write-Error "no indexes/tx files to corrupt"
        }
        $pick = $candidates[0]
        Invoke-TailTruncate -Path $pick.FullName -TruncateBy 12 -MinSize 36
    }
}

& "$PSScriptRoot\restart_node.ps1" -DataDir $DataDir -Network $Network -WaitSec $WaitSec
if ($LASTEXITCODE -ne 0) {
    Write-Host "restart_node returned non-zero; continuing RPC probe" -ForegroundColor Yellow
}

$deadline = (Get-Date).AddSeconds($WaitSec)
$recovered = $false
$afterInfo = $null
do {
    Start-Sleep -Seconds 2
    try {
        $afterInfo = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 2 -WarmupDelaySec 1
        if ($null -ne $afterInfo.headers -and $afterInfo.headers -ge 0) {
            $recovered = $true
            break
        }
    } catch {
        if ($_.Exception.Message -match "warming up|-28") {
            Write-Host "RPC warming up after corruption..." -ForegroundColor DarkGray
        }
    }
} while ((Get-Date) -lt $deadline)

if (-not $recovered) {
    Write-Error "Node did not recover RPC within ${WaitSec}s after $Target corruption"
}

Write-Host ("Recovered: headers={0} blocks={1} ibd={2}" -f $afterInfo.headers, $afterInfo.blocks, $afterInfo.initialblockdownload) -ForegroundColor Green

try {
    $vc = Invoke-DogeGoJsonRpc -Method verifychain -Params @(2, 0) -WarmupRetries 2 -WarmupDelaySec 1
    if ($null -eq $vc -or $vc -eq $false) {
        Write-Error "verifychain 2 0 failed after $Target corruption recovery"
    }
    Write-Host "verifychain 2 0: ok" -ForegroundColor DarkGray
} catch {
    Write-Error "verifychain unavailable after recovery: $_"
}

if ($Target -eq "txindex") {
    try {
        $ix = Invoke-DogeGoJsonRpc -Method getindexinfo -WarmupRetries 1 -WarmupDelaySec 1
        if ($null -eq $ix) {
            Write-Warning "getindexinfo returned empty after txindex corruption"
        } else {
            Write-Host "getindexinfo: ok" -ForegroundColor DarkGray
        }
    } catch {
        Write-Warning "getindexinfo probe failed: $_"
    }
}

if ($beforeInfo -and $afterInfo.headers -lt ($beforeInfo.headers - 1000)) {
    Write-Warning "Header count dropped significantly; check journal repair logs"
}
Write-Host "Live corruption inject probe passed ($Target)." -ForegroundColor Green
exit 0
