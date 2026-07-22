# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Verify a DogeGo build has Core-parity aux parent chain ID rules and (optionally) headers past 510k.
# Run after stopping the old node and starting the new dogego.exe.
#
#   .\scripts\upgrade_post_aux_verify.ps1
#   .\scripts\upgrade_post_aux_verify.ps1 -RequireHeadersPast510k
param(
    [switch]$RequireHeadersPast510k,
    [string]$RpcUser,
    [string]$RpcPassword,
    [string]$RpcHost,
    [int]$RpcPort = 0,
    [int]$WatchSec = 0
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

$rpcParams = @{}
if ($RpcUser) { $rpcParams.RpcUser = $RpcUser }
if ($RpcPassword) { $rpcParams.RpcPassword = $RpcPassword }
if ($RpcHost) { $rpcParams.RpcHost = $RpcHost }
if ($RpcPort -gt 0) { $rpcParams.RpcPort = $RpcPort }

function Invoke-BlockchainInfo {
    return Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 8 -WarmupDelaySec 3 @rpcParams
}

Write-Host "=== Post-aux upgrade verification ===" -ForegroundColor Cyan
try {
    $info = Invoke-BlockchainInfo
} catch {
    Write-Host "FAIL: $_" -ForegroundColor Red
    Write-Host "Start: dogego node  (JSON-RPC must be listening - default http://127.0.0.1:22557/)" -ForegroundColor DarkGray
    exit 2
}
if ($info.PSObject.Properties.Name -notcontains "dogego_auxpow_parent_chain_id_core_parity" -or
    $info.dogego_auxpow_parent_chain_id_core_parity -ne $true) {
    Write-Host "FAIL: dogego_auxpow_parent_chain_id_core_parity is not true - rebuild from current DogeGo sources" -ForegroundColor Red
    exit 1
}
Write-Host "OK: dogego_auxpow_parent_chain_id_core_parity=true" -ForegroundColor Green
Write-Host ("headers={0} blocks={1} ibd={2}" -f $info.headers, $info.blocks, $info.initialblockdownload)
if ($info.PSObject.Properties.Name -contains "dogego_post_aux_era_header_stall" -and $info.dogego_post_aux_era_header_stall -eq $true) {
    Write-Host "NOTE: dogego_post_aux_era_header_stall=true - headers should advance within minutes on this build" -ForegroundColor Yellow
}
if ($info.PSObject.Properties.Name -contains "warnings" -and $info.warnings) {
    Write-Host ("warnings: {0}" -f $info.warnings) -ForegroundColor Yellow
}
if ($RequireHeadersPast510k) {
    $chkArgs = @{}
    if ($RpcUser) { $chkArgs.RpcUser = $RpcUser }
    if ($RpcPassword) { $chkArgs.RpcPassword = $RpcPassword }
    if ($RpcHost) { $chkArgs.RpcHost = $RpcHost }
    if ($RpcPort -gt 0) { $chkArgs.RpcPort = $RpcPort }
    & "$PSScriptRoot\check_header_progress.ps1" @chkArgs
    exit $LASTEXITCODE
}
if ($WatchSec -gt 0) {
    Write-Host "Watching headers for ${WatchSec}s..." -ForegroundColor Cyan
    $start = [int64]$info.headers
    Start-Sleep -Seconds $WatchSec
    $info2 = Invoke-BlockchainInfo
    if ([int64]$info2.headers -le $start) {
        Write-Host ("WARN: headers unchanged ({0}) - run .\scripts\watch_sync.ps1" -f $start) -ForegroundColor Yellow
        exit 1
    }
    Write-Host ("OK: headers advanced {0} -> {1}" -f $start, $info2.headers) -ForegroundColor Green
}
Write-Host "Upgrade verification passed." -ForegroundColor Green
exit 0
