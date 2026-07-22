# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Append one CSV row from live RPC (for Task Scheduler / cron).
# Does not loop - pair with log_ibd_progress.ps1 for continuous logging, or schedule this every minute.
#
#   .\scripts\ibd_snapshot.ps1
#   .\scripts\ibd_snapshot.ps1 -OutFile ibd_progress.csv
param(
    [string]$OutFile,
    [string]$DataDir,
    [string]$Network = "mainnet"
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"
$inv = [System.Globalization.CultureInfo]::InvariantCulture

function FmtNum($n) {
    if ($null -eq $n -or $n -eq "") { return "" }
    return ([string]::Format($inv, "{0}", $n))
}

$ts = (Get-Date).ToString("o")
$info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 5 -WarmupDelaySec 2
$cont = ""
$probe = ""
$connectLag = ""
$downloadPerMin = ""
$connectPerMin = ""
$hdrPct = ""
$stall = ""
$parity = ""
$warn = ""
if ($info.PSObject.Properties.Name -contains "dogego_contiguous_raw_height") {
    $cont = $info.dogego_contiguous_raw_height
}
$cl = Get-DogeGoRpcConnectLag $info
if ($null -ne $cl) { $connectLag = $cl }
if ($info.PSObject.Properties.Name -contains "dogego_raw_sync") {
    $rs = $info.dogego_raw_sync
    if ($rs.next_probe_height) { $probe = $rs.next_probe_height }
    if ($rs.blocks_per_minute) { $downloadPerMin = [math]::Round([double]$rs.blocks_per_minute, 2) }
}
$cRate = Get-DogeGoRpcConnectBlocksPerMinute $info
if ($null -ne $cRate) { $connectPerMin = [math]::Round($cRate, 2) }
$connectBoost = Format-DogeGoConnectCatchUpBoost (Get-DogeGoRpcConnectCatchUpTuning $info)
if ($null -eq $connectBoost) { $connectBoost = "" }
if ($info.PSObject.Properties.Name -contains "dogego_headers_sync_progress") {
    $hdrPct = [math]::Round([double]$info.dogego_headers_sync_progress * 100, 2)
}
$stall = if ($info.dogego_post_aux_era_header_stall) { "True" } else { "False" }
$parity = if ($info.dogego_auxpow_parent_chain_id_core_parity) { "True" } else { "False" }
if ($info.warnings) {
    if ($info.warnings -is [array]) { $warn = ($info.warnings -join "; ") -replace '"', '""' }
    else { $warn = ([string]$info.warnings) -replace '"', '""' }
}
$bodyPct = ""
$disk = Get-DogeGoDiskSyncSnapshot -DataDir $DataDir -Network $Network
if ($null -ne $disk.BodyPct) { $bodyPct = $disk.BodyPct }

$line = ('{0},rpc,{1},{2},{3},{4},{5},{6},{7},{8},{9},{10},{11},{12},"{13}","{14}"' -f $ts, $info.headers, $info.blocks, $info.initialblockdownload, (FmtNum $cont), (FmtNum $probe), (FmtNum $connectLag), (FmtNum $bodyPct), (FmtNum $downloadPerMin), (FmtNum $connectPerMin), (FmtNum $hdrPct), $stall, $parity, $warn, ($connectBoost -replace '"', '""'))

if ($OutFile) {
    $path = Join-Path (Get-Location) $OutFile
    if (-not (Test-Path $path)) {
        "timestamp,source,headers,blocks,ibd,contiguous_raw,raw_probe,connect_lag,body_pct,download_per_min,connect_per_min,hdr_progress,post_aux_stall,parity_flag,warnings,connect_boost" | Out-File -FilePath $path -Encoding utf8
    }
    Add-Content -Path $path -Value $line -Encoding utf8
    Write-Host "Appended to $path"
} else {
    Write-Host $line
}
