# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Compare committed canonical field blocks against Core or DogeGo RPC (read-only).
#
#   .\scripts\verify_mainnet_field_canonical.ps1
#   .\scripts\verify_mainnet_field_canonical.ps1 -Heights @(10006)
param(
    [int[]]$Heights = @(1, 2, 3, 100, 200, 272, 10006)
)
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

function Get-CommittedFieldHex([int]$Height) {
    $path = Join-Path $DogeGo "consensus\testdata\mainnet_field_blocks.json"
    if (-not (Test-Path $path)) { throw "missing $path" }
    $entries = Get-Content $path -Raw | ConvertFrom-Json
    foreach ($e in $entries) {
        if ([int]$e.height -eq $Height) {
            return ([string]$e.hex).ToUpper()
        }
    }
    return $null
}

function Invoke-CoreCliBlockHexLocal([int]$Height) {
    $coreCli = $env:DOGEGO_CORE_CLI
    if (-not $coreCli) {
        $coreCli = (Get-Command dogecoin-cli -ErrorAction SilentlyContinue).Source
    }
    if (-not $coreCli) {
        $default = "C:\Program Files\Dogecoin\daemon\dogecoin-cli.exe"
        if (Test-Path $default) { $coreCli = $default }
    }
    if (-not $coreCli) { throw "dogecoin-cli not found (set DOGEGO_CORE_CLI)" }
    $hash = (& $coreCli getblockhash $Height 2>&1 | Out-String).Trim().Trim('"')
    if ($LASTEXITCODE -ne 0) { throw "getblockhash $Height failed: $hash" }
    $hex = (& $coreCli getblock $hash 0 2>&1 | Out-String).Trim().Trim('"')
    if ($LASTEXITCODE -ne 0) { throw "getblock $Height failed: $hex" }
    return $hex.ToUpper()
}

function Invoke-DogeGoBlockHexLocal([int]$Height) {
    . "$PSScriptRoot\dogego_rpc.ps1"
    $hash = Invoke-DogeGoJsonRpc -Method getblockhash -Params @($Height) -WarmupRetries 2 -WarmupDelaySec 2
    if (-not $hash) { throw "empty hash for height $Height" }
    $hex = Invoke-DogeGoJsonRpc -Method getblock -Params @($hash, 0) -WarmupRetries 1
    if (-not $hex) { throw "empty block hex for height $Height" }
    return $hex.ToUpper()
}

$useCore = $false
try {
    . "$PSScriptRoot\dogego_rpc.ps1"
    $null = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 1
} catch {
    Write-Host "DogeGo RPC unavailable; using dogecoin-cli" -ForegroundColor Yellow
    $useCore = $true
}

$failed = 0
foreach ($h in $Heights) {
    $want = Get-CommittedFieldHex $h
    if (-not $want) {
        Write-Host "height $h : missing committed fixture" -ForegroundColor Red
        $failed++
        continue
    }
    $via = if ($useCore) { "Core" } else { "DogeGo" }
    $got = if ($useCore) { Invoke-CoreCliBlockHexLocal $h } else { Invoke-DogeGoBlockHexLocal $h }
    if ($got -eq $want) {
        Write-Host "height $h : OK ($via, $([int]($got.Length / 2)) B)" -ForegroundColor Green
    } else {
        Write-Host "height $h : MISMATCH ($via)" -ForegroundColor Red
        $failed++
    }
}

if ($failed -gt 0) {
    Write-Host "`n$failed height(s) differ from committed canonical fixtures." -ForegroundColor Red
    exit 1
}
Write-Host "`nAll committed field blocks match RPC." -ForegroundColor Green
exit 0
