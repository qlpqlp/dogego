# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: Core-style maintenance RPC workflow (live node required).
# Verifies verifychain, getindexinfo, getchaintxstats, and index/filter diagnostics vs Core shape.
#
#   .\scripts\core_maintenance_workflow.ps1
#   .\scripts\core_maintenance_workflow.ps1 -Json
param(
    [switch]$Json,
    [string]$DataDir,
    [string]$Network = "mainnet"
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

$issues = @()
$warnings = @()
$notes = @()

$coreCli = $env:DOGEGO_CORE_CLI
if (-not $coreCli) {
    $coreCli = (Get-Command dogecoin-cli -ErrorAction SilentlyContinue).Source
}

try {
    $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 5 -WarmupDelaySec 2
} catch {
    $issues += "rpc_unreachable"
    $info = $null
}

$indexInfo = $null
$chainStats = $null
$verifyOk = $null

if ($info) {
    try {
        $indexInfo = Invoke-DogeGoJsonRpc -Method getindexinfo
    } catch {
        $issues += "getindexinfo_failed"
    }
    try {
        $chainStats = Invoke-DogeGoJsonRpc -Method getchaintxstats -Params @(24)
    } catch {
        $warnings += "getchaintxstats_failed"
    }
    try {
        foreach ($level in @(2, 4)) {
            $verifyOk = Invoke-DogeGoJsonRpc -Method verifychain -Params @($level, 0)
            if ($verifyOk -ne $true -and "$verifyOk" -notmatch "true") {
                if ($info.initialblockdownload -eq $true) {
                    $warnings += "verifychain_${level}_not_true_during_ibd"
                } elseif ($level -eq 4) {
                    $issues += "verifychain_not_true"
                } else {
                    $warnings += "verifychain_${level}_not_true"
                }
            }
        }
    } catch {
        $issues += "verifychain_failed"
    }
    if ($info.blocks -gt $info.headers) {
        $issues += "blocks_exceed_headers"
    }
    if ($info.PSObject.Properties.Name -contains "dogego_filter_index_lag") {
        $lag = [int64]$info.dogego_filter_index_lag
        if ($lag -gt 10000 -and $info.initialblockdownload -eq $true) {
            $notes += "filter_index_lag_expected_during_ibd"
        }
    }
}

if ($indexInfo) {
    $hasTx = $indexInfo.PSObject.Properties.Name -contains "txindex"
    if (-not $hasTx) {
        $warnings += "getindexinfo_missing_txindex"
    }
    if ($indexInfo.PSObject.Properties.Name -contains "basic block filter" -and $indexInfo."basic block filter".synced -eq $false) {
        if ($info -and $info.initialblockdownload -eq $true) {
            $notes += "block_filter_index_catching_up"
        } else {
            $warnings += "block_filter_index_not_synced"
        }
    } elseif ($indexInfo.PSObject.Properties.Name -contains "basic" -and $indexInfo.basic.synced -eq $false) {
        if ($info -and $info.initialblockdownload -eq $true) {
            $notes += "block_filter_index_catching_up"
        } else {
            $warnings += "block_filter_index_not_synced"
        }
    }
}

if ($chainStats) {
    if ($chainStats.PSObject.Properties.Name -notcontains "window_tx_count") {
        $warnings += "getchaintxstats_missing_window_tx_count"
    }
    if ($info -and $chainStats.PSObject.Properties.Name -contains "time") {
        if ([int64]$chainStats.time -le 0) {
            $warnings += "getchaintxstats_time_zero"
        }
    }
}

if ($info -and [int64]$info.blocks -gt 0) {
    try {
        $best = Invoke-DogeGoJsonRpc -Method getbestblockhash
        $filter = Invoke-DogeGoJsonRpc -Method getblockfilter -Params @($best)
        if (-not $filter -or -not $filter.filter) {
            $warnings += "getblockfilter_empty_at_tip"
        } else {
            $notes += "getblockfilter_ok_at_tip"
            if ($coreCli -and $info.initialblockdownload -eq $false) {
                $corePort = if ($env:DOGEGO_CORE_RPC_PORT) { $env:DOGEGO_CORE_RPC_PORT } else { $(if ($env:DOGEGO_RPC_PORT) { $env:DOGEGO_RPC_PORT } else { "22557" }) }
                try {
                    $coreInfoArgs = @("-rpcport=$corePort", "getblockchaininfo")
                    if ($env:DOGEGO_CORE_RPC_USER) {
                        $coreInfoArgs = @("-rpcuser=$env:DOGEGO_CORE_RPC_USER", "-rpcpassword=$env:DOGEGO_CORE_RPC_PASS") + $coreInfoArgs
                    }
                    $coreInfo = & $coreCli @coreInfoArgs 2>&1 | ConvertFrom-Json
                    if ($coreInfo.bestblockhash -eq $best) {
                        $coreFilterArgs = @("-rpcport=$corePort", "getblockfilter", $best)
                        if ($env:DOGEGO_CORE_RPC_USER) {
                            $coreFilterArgs = @("-rpcuser=$env:DOGEGO_CORE_RPC_USER", "-rpcpassword=$env:DOGEGO_CORE_RPC_PASS") + $coreFilterArgs
                        }
                        $coreFilter = & $coreCli @coreFilterArgs 2>&1 | ConvertFrom-Json
                        if ($coreFilter.filter -and ($coreFilter.filter -eq $filter.filter)) {
                            $notes += "getblockfilter_tip_aligned"
                        } elseif ($coreFilter.filter) {
                            $warnings += "getblockfilter_tip_mismatch"
                        } else {
                            $warnings += "core_getblockfilter_empty_at_tip"
                        }
                    } else {
                        $notes += "getblockfilter_compare_skipped_tip_mismatch"
                    }
                } catch {
                    $notes += "core_getblockfilter_compare_skipped"
                }
            }
        }
    } catch {
        if ($info.initialblockdownload -eq $true) {
            $notes += "getblockfilter_skipped_during_ibd"
        } else {
            $warnings += "getblockfilter_failed"
        }
    }
}

if ($coreCli -and $info -and $info.initialblockdownload -eq $false) {
    $dgPort = if ($env:DOGEGO_RPC_PORT) { $env:DOGEGO_RPC_PORT } else { "22557" }
    $corePort = if ($env:DOGEGO_CORE_RPC_PORT) { $env:DOGEGO_CORE_RPC_PORT } else { $dgPort }
    foreach ($level in @(2, 4)) {
        try {
            $coreArgs = @("-rpcport=$corePort", "verifychain", "$level", "0")
            if ($env:DOGEGO_CORE_RPC_USER) {
                $coreArgs = @("-rpcuser=$env:DOGEGO_CORE_RPC_USER", "-rpcpassword=$env:DOGEGO_CORE_RPC_PASS") + $coreArgs
            }
            $dgV = Invoke-DogeGoJsonRpc -Method verifychain -Params @($level, 0) -RpcPort ([int]$dgPort) -WarmupRetries 1
            $coreV = & $coreCli @coreArgs 2>&1 | ConvertFrom-Json
            if ($coreV -ne $true -and "$coreV" -notmatch "true") {
                $warnings += "core_verifychain_${level}_not_true"
            } elseif ($dgV -eq $true -or "$dgV" -match "true") {
                $notes += "verifychain_${level}_core_ok"
            }
        } catch {
            $notes += "core_verifychain_${level}_compare_skipped"
        }
    }
    try {
        $coreArgs = @("-rpcport=$corePort", "getchaintxstats", "24")
        if ($env:DOGEGO_CORE_RPC_USER) {
            $coreArgs = @("-rpcuser=$env:DOGEGO_CORE_RPC_USER", "-rpcpassword=$env:DOGEGO_CORE_RPC_PASS") + $coreArgs
        }
        $coreStats = & $coreCli @coreArgs 2>&1 | ConvertFrom-Json
        if ($chainStats -and $coreStats) {
            $dgWin = [int64]$chainStats.window_tx_count
            $coreWin = [int64]$coreStats.txcount
            if ($coreStats.PSObject.Properties.Name -contains "window_tx_count") {
                $coreWin = [int64]$coreStats.window_tx_count
            }
            $delta = [Math]::Abs($dgWin - $coreWin)
            if ($delta -gt 500) {
                $warnings += "chaintxstats_window_delta_$delta"
            } else {
                $notes += "chaintxstats_window_aligned"
            }
        }
    } catch {
        $notes += "core_chaintxstats_compare_skipped"
    }
    if ($indexInfo) {
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
}

$ok = ($issues.Count -eq 0)
$report = [ordered]@{
    ok       = $ok
    network  = $Network
    blocks   = if ($info) { $info.blocks } else { $null }
    headers  = if ($info) { $info.headers } else { $null }
    ibd      = if ($info) { $info.initialblockdownload } else { $null }
    verifychain = $verifyOk
    index    = $indexInfo
    chaintxstats = $chainStats
    issues   = @($issues)
    warnings = @($warnings)
    notes    = @($notes)
}

if ($Json) {
    $report | ConvertTo-Json -Depth 8
} else {
    Write-Host "=== Core maintenance workflow ===" -ForegroundColor Cyan
    if ($info) {
        Write-Host ("blocks={0} headers={1} ibd={2}" -f $info.blocks, $info.headers, $info.initialblockdownload)
    }
    Write-Host ("verifychain 4 0: {0}" -f $verifyOk)
    foreach ($w in $warnings) { Write-Host ("WARN: " + $w) -ForegroundColor Yellow }
    foreach ($i in $issues) { Write-Host ("FAIL: " + $i) -ForegroundColor Red }
    foreach ($n in $notes) { Write-Host ("NOTE: " + $n) -ForegroundColor DarkGray }
    if ($ok) {
        Write-Host "`nCore maintenance workflow passed." -ForegroundColor Green
    } else {
        Write-Host "`nCore maintenance workflow failed." -ForegroundColor Red
    }
}

if (-not $ok) { exit 1 }
exit 0
