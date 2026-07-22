# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone D: offline BIP125 rule 2 + rule 5 corpus rows (no live node).
#
#   .\scripts\core_mempool_bip125_offline_probe.ps1
#   .\scripts\core_mempool_bip125_offline_probe.ps1 -Json
param(
    [switch]$Json
)
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

$templates = @(
    "rbf_too_many_conflicts",
    "rbf_new_unconfirmed_input"
)

if (-not $Json) {
    Write-Host "=== BIP125 offline corpus (rule 2 + rule 5) ===" -ForegroundColor Cyan
}

$failed = 0
foreach ($tmpl in $templates) {
    if (-not $Json) {
        Write-Host ("`n[{0}]" -f $tmpl) -ForegroundColor Yellow
    }
    go test ./consensus -run ("TestCoreMempoolDifferentialVectors/" + $tmpl) -count=1
    if ($LASTEXITCODE -ne 0) {
        $failed++
        if (-not $Json) {
            Write-Host "  FAIL" -ForegroundColor Red
        }
    } elseif (-not $Json) {
        Write-Host "  OK" -ForegroundColor Green
    }
}

if ($Json) {
    [ordered]@{
        ok        = ($failed -eq 0)
        templates = $templates.Count
        failed    = $failed
    } | ConvertTo-Json -Compress
}

if ($failed -gt 0) {
    if (-not $Json) {
        Write-Host "`nBIP125 offline probe failed." -ForegroundColor Red
    }
    exit 1
}
if (-not $Json) {
    Write-Host "`nBIP125 offline probe passed." -ForegroundColor Green
}
exit 0
