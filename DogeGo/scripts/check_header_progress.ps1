# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Exit 0 when mainnet header tip has passed the post-aux era stall band (510k+).
# Usage:
#   .\scripts\check_header_progress.ps1
#   .\scripts\check_header_progress.ps1 -MinHeaders 600000
param(
    [int64]$MinHeaders = 510000,
    [string]$RpcUser,
    [string]$RpcPassword,
    [string]$RpcHost,
    [int]$RpcPort = 0
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

$rpcParams = @{}
if ($RpcUser) { $rpcParams.RpcUser = $RpcUser }
if ($RpcPassword) { $rpcParams.RpcPassword = $RpcPassword }
if ($RpcHost) { $rpcParams.RpcHost = $RpcHost }
if ($RpcPort -gt 0) { $rpcParams.RpcPort = $RpcPort }

try {
    $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo @rpcParams
} catch {
    Write-Host "getblockchaininfo failed: $_" -ForegroundColor Red
    Write-Host "Is dogego node running? JSON-RPC uses HTTP POST (see scripts/dogego_rpc.ps1)." -ForegroundColor DarkGray
    exit 2
}
$headers = [int64]$info.headers
Write-Host ("headers={0} blocks={1} ibd={2}" -f $headers, $info.blocks, $info.initialblockdownload)
if ($info.PSObject.Properties.Name -contains "dogego_post_aux_era_header_stall" -and $info.dogego_post_aux_era_header_stall -eq $true) {
    Write-Host "dogego_post_aux_era_header_stall=true - rebuild dogego.exe if aux chain-id errors persist" -ForegroundColor Yellow
}
if ($info.PSObject.Properties.Name -contains "dogego_header_sync_recovery" -and $info.dogego_header_sync_recovery) {
    Write-Host ("recovery: {0}" -f $info.dogego_header_sync_recovery) -ForegroundColor Yellow
}
if ($headers -lt $MinHeaders) {
    Write-Host ("FAIL: headers {0} < {1} (post-aux era checkpoint not reached)" -f $headers, $MinHeaders) -ForegroundColor Red
    exit 1
}
Write-Host ("OK: headers >= {0}" -f $MinHeaders) -ForegroundColor Green
exit 0
