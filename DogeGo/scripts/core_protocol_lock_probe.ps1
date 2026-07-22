# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Solo protocol-lock sanity: DogeGo getdeploymentinfo + getblockchaininfo softforks vs consensus activation heights.
# Does not require Dogecoin Core. Mirrors GET /api/core-compare deployment.protocol_lock.
#
#   cd DogeGo
#   .\scripts\core_protocol_lock_probe.ps1
param(
    [string]$RpcPort = $(if ($env:DOGEGO_RPC_PORT) { $env:DOGEGO_RPC_PORT } else { "22557" }),
    [string]$RpcUser = $(if ($env:DOGEGO_RPC_USER) { $env:DOGEGO_RPC_USER } else { "dogego" }),
    [string]$RpcPassword = $(if ($env:DOGEGO_RPC_PASS) { $env:DOGEGO_RPC_PASS } else { "dogego" })
)
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo
. "$PSScriptRoot\dogego_rpc.ps1"

function Get-ExpectedActive($chain, $height, $name) {
    $h = [int64]$height
    if ($chain -eq "main") {
        switch ($name) {
            "bip34" { return $h -ge 1034383 }
            "bip66" { return $h -ge 1034383 }
            "bip65" { return $h -ge 3464751 }
            "csv"   { return $h -ge 419328 }
            default { return $null }
        }
    }
    if ($chain -eq "test") {
        switch ($name) {
            "bip34" { return $h -ge 708658 }
            "bip66" { return $h -ge 708658 }
            "bip65" { return $h -ge 1854705 }
            "csv"   { return $h -ge 708658 }
            default { return $null }
        }
    }
    return $null
}

Write-Host "=== DogeGo protocol-lock probe (solo) ===" -ForegroundColor Cyan
$rpcParams = @{ RpcPort = [int]$RpcPort; WarmupRetries = 15; WarmupDelaySec = 4 }
if ($RpcUser) { $rpcParams.RpcUser = $RpcUser }
if ($RpcPassword) { $rpcParams.RpcPassword = $RpcPassword }

$info = Invoke-DogeGoJsonRpc -Method getblockchaininfo @rpcParams
$chain = $info.chain
$dep = Invoke-DogeGoJsonRpc -Method getdeploymentinfo @rpcParams
$height = [int64]$dep.height
Write-Host ("chain={0} height={1}" -f $chain, $height)

$names = @("bip34", "bip66", "bip65", "csv")
$lockOk = $true
foreach ($name in $names) {
    $expected = Get-ExpectedActive $chain $height $name
    if ($null -eq $expected) {
        Write-Host "WARN: unknown chain $chain for deployment $name" -ForegroundColor Yellow
        continue
    }
    $actual = $false
    if ($dep.deployments.$name) {
        $actual = [bool]$dep.deployments.$name.active
    }
    $match = ($actual -eq $expected)
    if (-not $match) { $lockOk = $false }
    Write-Host ("deployment.{0}.active: actual={1} expected={2} match={3}" -f $name, $actual, $expected, $match)
}

if ($info.softforks) {
    foreach ($sf in @($info.softforks)) {
        $name = $sf.id
        if (-not $name) { continue }
        $expected = Get-ExpectedActive $chain $height $name
        if ($null -eq $expected) { continue }
        $actual = $false
        if ($sf.reject) { $actual = [bool]$sf.reject.status }
        $match = ($actual -eq $expected)
        if (-not $match) { $lockOk = $false }
        Write-Host ("softfork.{0}.reject: actual={1} expected={2} match={3}" -f $name, $actual, $expected, $match)
    }
}
if ($info.bip9_softforks) {
    $bip9Names = @($info.bip9_softforks.PSObject.Properties.Name)
    foreach ($name in $bip9Names) {
        $expected = Get-ExpectedActive $chain $height $name
        if ($null -eq $expected) { continue }
        $status = $info.bip9_softforks.$name.status
        $actual = ($status -eq "active")
        $match = ($actual -eq $expected)
        if (-not $match) { $lockOk = $false }
        Write-Host ("bip9_softfork.{0}.active: actual={1} expected={2} match={3}" -f $name, $actual, $expected, $match)
    }
}

if (-not $lockOk) {
    Write-Host "FAIL: deployment.protocol_lock mismatch" -ForegroundColor Red
    exit 1
}
Write-Host "deployment.protocol_lock: OK" -ForegroundColor Green
exit 0
