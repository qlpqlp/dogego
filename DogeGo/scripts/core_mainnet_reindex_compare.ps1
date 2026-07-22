# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: read-only getindexinfo / reindex RPC presence compare (Core vs DogeGo on mainnet).
#
#   .\scripts\core_mainnet_reindex_compare.ps1
param(
    [switch]$Json,
    [string]$DogeGoRpcPort = "22557",
    [string]$CoreRpcPort = "22555"
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

$coreCli = $env:DOGEGO_CORE_CLI
if (-not $coreCli) {
    $coreCli = (Get-Command dogecoin-cli -ErrorAction SilentlyContinue).Source
}
if (-not $coreCli) {
    $d = "C:\Program Files\Dogecoin\daemon\dogecoin-cli.exe"
    if (Test-Path $d) { $coreCli = $d }
}

$issues = @()
$warnings = @()
$notes = @()

function Invoke-DgIdx {
    $rpc = Invoke-DogeGoJsonRpc -Method getindexinfo -RpcPort ([int]$DogeGoRpcPort) -WarmupRetries 3
    $methods = Invoke-DogeGoJsonRpc -Method getrpcinfo -RpcPort ([int]$DogeGoRpcPort) -WarmupRetries 2
    return @{ index = $rpc; rpc = $methods }
}

function Invoke-CoreIdx {
    if (-not $coreCli) { return $null }
    $args = @("-rpcport=$CoreRpcPort", "getindexinfo")
    if ($env:DOGEGO_CORE_RPC_USER) {
        $args = @("-rpcuser=$env:DOGEGO_CORE_RPC_USER", "-rpcpassword=$env:DOGEGO_CORE_RPC_PASS") + $args
    }
    $idx = & $coreCli @args 2>&1
    if ($LASTEXITCODE -ne 0) { return $null }
    return @{ index = ($idx | ConvertFrom-Json) }
}

Write-Host "=== Mainnet reindex/index compare (read-only) ===" -ForegroundColor Cyan

$dg = Invoke-DgIdx
$core = Invoke-CoreIdx

$required = @("reindextx", "reindexblockfilters", "pruneblockchain", "getindexinfo")
if ($dg.rpc -and $dg.rpc.method) {
    foreach ($m in $required) {
        if ($dg.rpc.method.PSObject.Properties.Name -notcontains $m) {
            $issues += "dogego_rpc_missing_$m"
        }
    }
}

if ($dg.index) {
    foreach ($key in @("txindex", "basic")) {
        if ($dg.index.PSObject.Properties.Name -contains $key) {
            $notes += "dogego_has_$key"
        }
    }
}

if ($core -and $core.index) {
    foreach ($key in @("txindex", "basic")) {
        $dgHas = $dg.index.PSObject.Properties.Name -contains $key
        $coreHas = $core.index.PSObject.Properties.Name -contains $key
        if ($dgHas -ne $coreHas) { $warnings += "index_${key}_presence_mismatch" }
    }
    if ($dg.index.txindex -and $core.index.txindex) {
        $notes += "txindex_synced_dg_$($dg.index.txindex.synced)_core_$($core.index.txindex.synced)"
    }
} elseif (-not $coreCli) {
    $notes += "core_cli_absent"
}

$ok = ($issues.Count -eq 0)
$report = [ordered]@{
    ok       = $ok
    dogego   = $dg.index
    core     = if ($core) { $core.index } else { $null }
    issues   = @($issues)
    warnings = @($warnings)
    notes    = @($notes)
}

if ($Json) {
    $report | ConvertTo-Json -Depth 6
} else {
    foreach ($w in $warnings) { Write-Host ("WARN: " + $w) -ForegroundColor Yellow }
    foreach ($i in $issues) { Write-Host ("FAIL: " + $i) -ForegroundColor Red }
    foreach ($n in $notes) { Write-Host ("NOTE: " + $n) -ForegroundColor DarkGray }
    if ($ok) { Write-Host "`nReindex compare passed." -ForegroundColor Green }
    else { Write-Host "`nReindex compare failed." -ForegroundColor Red }
}

if (-not $ok) { exit 1 }
exit 0
