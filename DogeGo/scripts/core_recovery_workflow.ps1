# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: recovery workflow probe (read-only by default).
# Checks dogego_recoverheaders RPC presence + chain recovery fields; optional -InvokeRecover (disruptive).
#
#   .\scripts\core_recovery_workflow.ps1
#   .\scripts\core_recovery_workflow.ps1 -InvokeRecover
param(
    [switch]$Json,
    [switch]$InvokeRecover,
    [string]$DataDir = "dogedata",
    [string]$Network = "reboottestnet"
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

$issues = @()
$notes = @()

try {
    $rpcInfo = Invoke-DogeGoJsonRpc -Method getrpcinfo -WarmupRetries 3 -WarmupDelaySec 2
} catch {
    $issues += "getrpcinfo_failed"
    $rpcInfo = $null
}

if ($rpcInfo -and $rpcInfo.method) {
    if (-not $rpcInfo.method.PSObject.Properties.Name -contains "dogego_recoverheaders") {
        $issues += "rpc_method_missing_dogego_recoverheaders"
    }
}

try {
    $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 3 -WarmupDelaySec 2
    if ($info.PSObject.Properties.Name -contains "dogego_header_sync_recovery" -and $info.dogego_header_sync_recovery) {
        $notes += ("header_sync_recovery=" + $info.dogego_header_sync_recovery)
    }
    if ($info.PSObject.Properties.Name -contains "dogego_header_catch_up_pending" -and $info.dogego_header_catch_up_pending) {
        $notes += "header_catch_up_pending=true"
    }
} catch {
    $issues += "getblockchaininfo_failed"
}

if ($InvokeRecover) {
    if ($Network -eq "mainnet") {
        Write-Error "InvokeRecover is reboottestnet/testnet only (disruptive header journal rewind)."
    }
    Write-Host "Invoking dogego_recoverheaders (disruptive) ..." -ForegroundColor Yellow
    try {
        $res = Invoke-DogeGoJsonRpc -Method dogego_recoverheaders -WarmupRetries 2 -WarmupDelaySec 2
        $notes += ("recoverheaders=" + ($res | ConvertTo-Json -Compress))
    } catch {
        $issues += "dogego_recoverheaders_failed"
    }
}

$ok = ($issues.Count -eq 0)
if ($Json) {
    [ordered]@{ ok = $ok; network = $Network; issues = $issues; notes = $notes } | ConvertTo-Json -Depth 4
} else {
    if ($ok) {
        Write-Host "Recovery workflow probe passed." -ForegroundColor Green
        foreach ($n in $notes) { Write-Host ("  note: " + $n) -ForegroundColor DarkGray }
    } else {
        Write-Host "Recovery workflow probe failed:" -ForegroundColor Red
        foreach ($i in $issues) { Write-Host ("  - " + $i) -ForegroundColor Red }
    }
}

if (-not $ok) { exit 1 }
exit 0
