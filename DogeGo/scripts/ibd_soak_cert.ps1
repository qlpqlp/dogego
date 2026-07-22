# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Optional long-haul / network certification for DogeGo standalone node parity.
# Offline gates always run; set DOGEGO_IBD_SOAK=1 to add live mainnet checks (requires dogego.exe + datadir).
#
#   cd DogeGo
#   .\scripts\ibd_soak_cert.ps1
#   $env:DOGEGO_IBD_SOAK = "1"
#   .\scripts\ibd_soak_cert.ps1
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

Write-Host "=== DogeGo IBD / soak certification ===" -ForegroundColor Cyan

Write-Host "`n[1/2] Offline operator + chainstate gates" -ForegroundColor Yellow
& "$PSScriptRoot\operator_workflow_cert.ps1"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if ($env:DOGEGO_IBD_SOAK -ne "1") {
    Write-Host "`n[2/2] Live mainnet soak skipped (set DOGEGO_IBD_SOAK=1 to enable)." -ForegroundColor DarkGray
    Write-Host "Optional: DOGEGO_CORE_COMPARE=1 with ibd_soak runs scripts/core_parity_probe.ps1 vs dogecoin-cli." -ForegroundColor DarkGray
    Write-Host "Optional: DOGEGO_IBD_CONVERGE=1 adds scripts/ibd_convergence_check.ps1 (2min body progress window)." -ForegroundColor DarkGray
    Write-Host "Optional: DOGEGO_CHECK_510K=1 with live soak requires headers >= 510000 (post-aux era)." -ForegroundColor DarkGray
    Write-Host "Optional: .\scripts\log_ibd_progress.ps1 -OutFile ibd_progress.csv for long-haul CSV logging." -ForegroundColor DarkGray
    Write-Host "When enabled, verify manually: getblockchaininfo, verifychain 4 0, gettxoutsetinfo after IBD." -ForegroundColor DarkGray
    Write-Host "`nOffline IBD/soak certification passed." -ForegroundColor Green
    exit 0
}

$dogego = Join-Path $DogeGo "dogego.exe"
if (-not (Test-Path $dogego)) {
    Write-Host "dogego.exe not found; build with: go build -o dogego.exe ./cmd/dogego" -ForegroundColor Red
    exit 1
}

Write-Host "`n[2/2] Live RPC health (mainnet datadir from dogecoinconf.json)" -ForegroundColor Yellow
& "$PSScriptRoot\node_health.ps1"
if ($LASTEXITCODE -ge 2) {
    Write-Host "node_health failed - see issues above" -ForegroundColor Red
    exit $LASTEXITCODE
}
& "$PSScriptRoot\upgrade_post_aux_verify.ps1"
if ($LASTEXITCODE -ne 0) {
    Write-Host "upgrade_post_aux_verify failed - rebuild dogego.exe from current sources" -ForegroundColor Red
    exit $LASTEXITCODE
}
. "$PSScriptRoot\dogego_rpc.ps1"
try {
    $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 8 -WarmupDelaySec 3
} catch {
    Write-Host "getblockchaininfo failed (is the node running on mainnet?): $_" -ForegroundColor Red
    exit 1
}
Write-Host ("headers={0} blocks={1} ibd={2}" -f $info.headers, $info.blocks, $info.initialblockdownload)
if ($info.PSObject.Properties.Name -contains "dogego_headers_sync_progress") {
    Write-Host ("headers_sync_progress={0:P2}" -f [double]$info.dogego_headers_sync_progress)
}
if ($info.PSObject.Properties.Name -contains "dogego_body_verification_progress") {
    Write-Host ("body_verification_progress={0:P2}" -f [double]$info.dogego_body_verification_progress)
}
if ($info.PSObject.Properties.Name -contains "dogego_contiguous_raw_height") {
    Write-Host ("contiguous_raw_height={0}" -f $info.dogego_contiguous_raw_height)
}
$connectLag = Get-DogeGoRpcConnectLag $info
$connectRate = Get-DogeGoRpcConnectBlocksPerMinute $info
if ($null -ne $connectLag) {
    Write-Host ("connect_lag={0} (stored ahead of chainActive)" -f $connectLag) -ForegroundColor DarkGray
    if ($info.PSObject.Properties.Name -contains "dogego_connect_catch_up_passes") {
        Write-Host ("connect_boost passes={0} batch={1} interval_ms={2}" -f $info.dogego_connect_catch_up_passes, $info.dogego_connect_catch_up_batch, $info.dogego_connect_catch_up_interval_ms) -ForegroundColor DarkGray
    }
}
if ($null -ne $connectRate) {
    Write-Host ("connect_rate={0:N1} blocks/min" -f $connectRate) -ForegroundColor DarkGray
}
if ($info.PSObject.Properties.Name -contains "dogego_raw_sync" -and $info.dogego_raw_sync.blocks_per_minute) {
    Write-Host ("download_rate={0:N1} blocks/min" -f [double]$info.dogego_raw_sync.blocks_per_minute) -ForegroundColor DarkGray
}
if ($null -ne $connectLag -and $connectLag -gt 5000) {
    if ($connectRate -gt 50) {
        $etaMin = [math]::Ceiling($connectLag / $connectRate)
        Write-Host ("OK: connect catch-up active (~{0} min to clear lag at current rate)" -f $etaMin) -ForegroundColor Green
    } elseif ($info.initialblockdownload -eq $true) {
        Write-Host "WARN: large connect lag with low connect rate - check logs for connect errors" -ForegroundColor Yellow
    }
}
if ($info.headers -gt 0 -and $info.blocks -ge 0) {
    $lag = [int64]$info.headers - [int64]$info.blocks
    if ($lag -gt 1000) {
        Write-Host ("header/body lag={0} blocks (normal during IBD)" -f $lag) -ForegroundColor DarkGray
    }
}
if ($info.PSObject.Properties.Name -contains "dogego_header_catch_up_pending" -and $info.dogego_header_catch_up_pending -eq $true) {
    Write-Host "NOTE: header catch-up still pending - watch logs for auxpow/header errors after upgrade" -ForegroundColor Yellow
}
if ($info.PSObject.Properties.Name -contains "dogego_header_sync_recovery" -and $info.dogego_header_sync_recovery) {
    Write-Host ("header_sync_recovery: {0}" -f $info.dogego_header_sync_recovery) -ForegroundColor Yellow
    if ($info.dogego_header_sync_recovery -match "outdated auxpow") {
        Write-Host "FAIL: rebuild dogego.exe (Core-parity aux parent chain id) and restart - see docs/CORE_OPERATOR_RUNBOOK.md" -ForegroundColor Red
        exit 1
    }
}
if ($info.initialblockdownload -eq $true -and $info.headers -ge 500000 -and $info.headers -lt 510000) {
    Write-Host "WARNING: headers still below 510k on mainnet IBD - confirm you are on a current build if this persists >30 min" -ForegroundColor Yellow
}
if ($info.initialblockdownload -eq $true -and $info.headers -ge 510000) {
    Write-Host "OK: headers past post-aux era (510k+)" -ForegroundColor Green
}
if ($env:DOGEGO_CHECK_510K -eq "1") {
    & "$PSScriptRoot\check_header_progress.ps1"
    if ($LASTEXITCODE -ne 0) {
        Write-Host "check_header_progress failed (set DOGEGO_CHECK_510K=0 to skip)" -ForegroundColor Red
        exit $LASTEXITCODE
    }
}
if ($info.PSObject.Properties.Name -contains "dogego_genesis_missing" -and $info.dogego_genesis_missing -eq $true) {
    Write-Host "WARNING: dogego_genesis_missing=true - block bodies cannot advance" -ForegroundColor Yellow
}
$count = $info.blocks
Write-Host "getblockcount (chainActive): $count"

try {
    $verify = Invoke-DogeGoJsonRpc -Method verifychain -Params @(4, 0) -WarmupRetries 3 -WarmupDelaySec 2
} catch {
    Write-Host "verifychain failed: $_" -ForegroundColor Red
    exit 1
}
Write-Host "verifychain 4 0: $verify"

if ($env:DOGEGO_CORE_COMPARE -eq "1") {
    Write-Host "`n[3/4] Core side-by-side RPC probe" -ForegroundColor Yellow
    & "$PSScriptRoot\core_parity_probe.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_MEMPOOL_PROBE -eq "1") {
    Write-Host "`n[4/4] Core testmempoolaccept parity (stateless corpus)" -ForegroundColor Yellow
    & "$PSScriptRoot\core_mempool_parity_probe.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} elseif ($env:DOGEGO_CORE_COMPARE -eq "1") {
    Write-Host "`nOptional: DOGEGO_MEMPOOL_PROBE=1 adds testmempoolaccept side-by-side checks." -ForegroundColor DarkGray
}

if ($env:DOGEGO_IBD_CONVERGE -eq "1") {
    Write-Host "`n[optional] IBD convergence (forward body progress)" -ForegroundColor Yellow
    & "$PSScriptRoot\ibd_convergence_check.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Write-Host "`nIBD/soak certification passed (offline + live RPC probes)." -ForegroundColor Green
