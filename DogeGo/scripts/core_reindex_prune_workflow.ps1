# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: Core-style reindex / index maintenance workflow (live node).
# Default: check-only (no destructive RPC). Use -RunReindex on testnet/dev only.
#
#   .\scripts\core_reindex_prune_workflow.ps1
#   .\scripts\core_reindex_prune_workflow.ps1 -Network testnet -RunReindex
param(
    [switch]$Json,
    [switch]$RunReindex,
    [string]$DataDir,
    [string]$Network = "mainnet"
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

$issues = @()
$warnings = @()
$notes = @()

$requiredMethods = @("reindextx", "reindexblockfilters", "pruneblockchain", "getindexinfo", "verifychain")

try {
    $rpcInfo = Invoke-DogeGoJsonRpc -Method getrpcinfo -WarmupRetries 3 -WarmupDelaySec 1
} catch {
    $issues += "getrpcinfo_failed"
    $rpcInfo = $null
}

if ($rpcInfo -and $rpcInfo.method) {
    foreach ($m in $requiredMethods) {
        if (-not $rpcInfo.method.PSObject.Properties.Name -contains $m) {
            $issues += ("rpc_method_missing_" + $m)
        }
    }
}

$indexInfo = $null
$info = $null
try {
    $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 5 -WarmupDelaySec 2
    $indexInfo = Invoke-DogeGoJsonRpc -Method getindexinfo
} catch {
    $issues += "rpc_chain_or_index_failed"
}

if ($indexInfo -and $info) {
    if ($indexInfo.PSObject.Properties.Name -contains "txindex" -and $indexInfo.txindex.synced -eq $false) {
        if ($info.initialblockdownload -eq $true) {
            $notes += "txindex_catching_up_during_ibd"
        } else {
            $warnings += "txindex_not_synced"
        }
    }
    if ($indexInfo.PSObject.Properties.Name -contains "basic block filter" -and $indexInfo."basic block filter".synced -eq $false) {
        if ($info.initialblockdownload -eq $true) {
            $notes += "block_filter_index_catching_up"
        } else {
            $warnings += "block_filter_index_not_synced"
        }
    } elseif ($indexInfo.PSObject.Properties.Name -contains "basic" -and $indexInfo.basic.synced -eq $false -and $info.initialblockdownload -ne $true) {
        $warnings += "block_filter_index_not_synced"
    }
}

$coreCli = $env:DOGEGO_CORE_CLI
if (-not $coreCli) {
    $coreCli = (Get-Command dogecoin-cli -ErrorAction SilentlyContinue).Source
}
if ($coreCli -and $indexInfo -and $info -and $info.initialblockdownload -eq $false) {
    $dgPort = if ($env:DOGEGO_RPC_PORT) { $env:DOGEGO_RPC_PORT } else { "22557" }
    $corePort = if ($env:DOGEGO_CORE_RPC_PORT) { $env:DOGEGO_CORE_RPC_PORT } else { $dgPort }
    try {
        $coreArgs = @("-rpcport=$corePort", "getindexinfo")
        if ($env:DOGEGO_CORE_RPC_USER) {
            $coreArgs = @("-rpcuser=$env:DOGEGO_CORE_RPC_USER", "-rpcpassword=$env:DOGEGO_CORE_RPC_PASS") + $coreArgs
        }
        $coreIdx = & $coreCli @coreArgs 2>&1 | ConvertFrom-Json
        foreach ($key in @("txindex", "basic")) {
            $dgSub = $null
            $coreSub = $null
            if ($key -eq "basic") {
                if ($indexInfo.PSObject.Properties.Name -contains "basic block filter") { $dgSub = $indexInfo."basic block filter" }
                elseif ($indexInfo.PSObject.Properties.Name -contains "basic") { $dgSub = $indexInfo.basic }
                if ($coreIdx.PSObject.Properties.Name -contains "basic block filter") { $coreSub = $coreIdx."basic block filter" }
                elseif ($coreIdx.PSObject.Properties.Name -contains "basic") { $coreSub = $coreIdx.basic }
            } else {
                if ($indexInfo.PSObject.Properties.Name -contains $key) { $dgSub = $indexInfo.$key }
                if ($coreIdx.PSObject.Properties.Name -contains $key) { $coreSub = $coreIdx.$key }
            }
            $dgHas = ($null -ne $dgSub)
            $coreHas = ($null -ne $coreSub)
            if ($dgHas -ne $coreHas) { $warnings += "getindexinfo_${key}_presence_mismatch" }
            if ($dgHas -and $coreHas -and ($dgSub.synced -ne $coreSub.synced)) {
                $warnings += "getindexinfo_${key}_synced_mismatch"
            }
        }
    } catch {
        $notes += "core_getindexinfo_compare_skipped"
    }
}

$reindexResult = $null
if ($RunReindex) {
    if ($Network -eq "mainnet") {
        $issues += "run_reindex_blocked_on_mainnet"
    } else {
        try {
            $before = Invoke-DogeGoJsonRpc -Method getindexinfo
            $reindexResult = Invoke-DogeGoJsonRpc -Method reindextx -Params @($false)
            $after = Invoke-DogeGoJsonRpc -Method getindexinfo
            if ($reindexResult.blocks_indexed -lt 0) {
                $warnings += "reindextx_unexpected_result"
            }
            $notes += "reindextx_executed_on_$Network"
        } catch {
            $issues += "reindextx_failed"
        }
    }
} else {
    $notes += "reindex_skipped_use_RunReindex_on_testnet"
}

$ok = ($issues.Count -eq 0)
$report = [ordered]@{
    ok            = $ok
    network       = $Network
    run_reindex   = [bool]$RunReindex
    blocks        = if ($info) { $info.blocks } else { $null }
    index         = $indexInfo
    reindex       = $reindexResult
    issues        = @($issues)
    warnings      = @($warnings)
    notes         = @($notes)
}

if ($Json) {
    $report | ConvertTo-Json -Depth 6
} else {
    Write-Host "=== Core reindex / maintenance workflow ===" -ForegroundColor Cyan
    foreach ($n in $notes) { Write-Host ("NOTE: " + $n) -ForegroundColor DarkGray }
    foreach ($w in $warnings) { Write-Host ("WARN: " + $w) -ForegroundColor Yellow }
    foreach ($i in $issues) { Write-Host ("FAIL: " + $i) -ForegroundColor Red }
    if ($ok) { Write-Host "`nReindex workflow check passed." -ForegroundColor Green }
    else { Write-Host "`nReindex workflow check failed." -ForegroundColor Red }
}

if (-not $ok) { exit 1 }
exit 0
