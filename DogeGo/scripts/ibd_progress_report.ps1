# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Summarize progress from ibd_progress.csv (or ibd_snapshot output).
# Usage:
#   .\scripts\ibd_progress_report.ps1
#   .\scripts\ibd_progress_report.ps1 -Csv ibd_progress.csv -LastRows 60
param(
    [string]$Csv = "ibd_progress.csv",
    [int]$LastRows = 30
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"
$path = Join-Path (Get-Location) $Csv
if (-not (Test-Path $path)) {
    Write-Host "CSV not found: $path" -ForegroundColor Red
    Write-Host "Start logging: .\scripts\log_ibd_progress.ps1 -OutFile ibd_progress.csv -IntervalSec 60"
    exit 1
}
$rows = Import-Csv $path
if ($rows.Count -lt 2) {
    Write-Host "Need at least 2 rows in $path (have $($rows.Count))" -ForegroundColor Yellow
    exit 1
}
if ($LastRows -gt 0 -and $rows.Count -gt $LastRows) {
    $rows = $rows[($rows.Count - $LastRows)..($rows.Count - 1)]
}
$a = $rows[0]
$b = $rows[-1]
function ToInt($v) {
    if ($null -eq $v -or $v -eq "") { return $null }
    return [int64]$v
}
function ToDbl($v) {
    if ($null -eq $v -or $v -eq "") { return $null }
    return [double]$v
}
$ah = ToInt $a.headers; $bh = ToInt $b.headers
$ab = ToInt $a.blocks; $bb = ToInt $b.blocks
$ac = ToInt $a.contiguous_raw; $bc = ToInt $b.contiguous_raw
$ap = ToInt $a.raw_probe; $bp = ToInt $b.raw_probe
$al = ToInt $a.connect_lag; $bl = ToInt $b.connect_lag
$ae = ToInt $a.body_eta_min; $be = ToInt $b.body_eta_min
Write-Host "=== IBD progress report ===" -ForegroundColor Cyan
Write-Host ("File: {0}  rows={1}  window: {2} -> {3}" -f $path, $rows.Count, $a.timestamp, $b.timestamp)
if ($null -ne $ah -and $null -ne $bh) {
    Write-Host ("headers: {0} -> {1}  ({2:+0;-0})" -f $ah, $bh, ($bh - $ah))
}
if ($null -ne $ab -and $null -ne $bb) {
    Write-Host ("chainActive (blocks): {0} -> {1}  ({2:+0;-0})" -f $ab, $bb, ($bb - $ab))
}
if ($null -ne $ac -and $null -ne $bc) {
    Write-Host ("stored (contiguous_raw): {0} -> {1}  ({2:+0;-0})" -f $ac, $bc, ($bc - $ac))
}
if ($null -ne $ap -and $null -ne $bp) {
    Write-Host ("probe (raw_probe): {0} -> {1}  ({2:+0;-0})" -f $ap, $bp, ($bp - $ap))
}
if ($null -ne $al -and $null -ne $bl) {
    Write-Host ("connect_lag: {0} -> {1}  ({2:+0;-0})" -f $al, $bl, ($bl - $al))
}
$dl = ToDbl $b.download_per_min
$cn = ToDbl $b.connect_per_min
if ($dl) { Write-Host ("latest download/min: {0:N1}" -f $dl) -ForegroundColor DarkGray }
if ($cn) { Write-Host ("latest connect/min: {0:N1}" -f $cn) -ForegroundColor DarkGray }
if ($b.body_pct) { Write-Host ("latest body_pct: {0}%" -f $b.body_pct) -ForegroundColor DarkGray }
if ($be -and $null -ne $bh -and $null -ne $bc -and $dl) {
    $etaText = Format-DogeGoBodyIBDEta -HeaderTip $bh -Contiguous $bc -BlocksPerMinute $dl
    if ($etaText) {
        Write-Host ("latest body ETA: {0} ({1} min)" -f $etaText, $be) -ForegroundColor DarkGray
    } else {
        Write-Host ("latest body_eta_min: {0}" -f $be) -ForegroundColor DarkGray
    }
} elseif ($be) {
    Write-Host ("latest body_eta_min: {0}" -f $be) -ForegroundColor DarkGray
}
if ($null -ne $bh -and $bh -ge 500000 -and $null -ne $bc) {
    $resumeCont = $bh - 50000
    if ($resumeCont -gt $bc) {
        $resumeBlocks = $resumeCont - $bc
        $resumeLine = "header resume at contiguous ~$resumeCont ($resumeBlocks blocks remaining)"
        if ($dl -gt 0) {
            $resumeEta = Format-DogeGoBodyIBDEta -HeaderTip $resumeCont -Contiguous $bc -BlocksPerMinute $dl
            if ($resumeEta) { $resumeLine += " ETA $resumeEta" }
        }
        Write-Host $resumeLine -ForegroundColor DarkGray
    }
}
if ($b.PSObject.Properties.Name -contains "connect_boost" -and $b.connect_boost) {
    Write-Host ("latest connect boost: {0}" -f $b.connect_boost) -ForegroundColor DarkGray
}
$forward = ($null -ne $bb -and $null -ne $ab -and $bb -gt $ab) -or
           ($null -ne $bc -and $null -ne $ac -and $bc -gt $ac) -or
           ($null -ne $bp -and $null -ne $ap -and $bp -gt $ap)
if ($forward) {
    Write-Host "Overall: forward progress in window" -ForegroundColor Green
    exit 0
}
Write-Host "Overall: no forward progress in window (node stopped or stalled?)" -ForegroundColor Yellow
exit 1
