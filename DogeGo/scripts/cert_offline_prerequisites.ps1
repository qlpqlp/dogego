# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# ROADMAP certification exit checklist - offline prerequisites bundle (no dogego-live).
#
# Cross-platform:
#   go run ./cmd/dogego cert offline && go run ./cmd/dogego cert wallet-import
#
#   .\scripts\cert_offline_prerequisites.ps1
#   .\scripts\cert_offline_prerequisites.ps1 -IncludePQ
#   .\scripts\cert_offline_prerequisites.ps1 -IncludeOperator
param(
    [switch]$IncludePQ,
    [switch]$IncludeOperator
)
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

Write-Host "=== DogeGo offline prerequisites (ROADMAP exit checklist) ===" -ForegroundColor Cyan

Write-Host "`n> go run ./cmd/dogego cert offline" -ForegroundColor Yellow
go run ./cmd/dogego cert offline
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "`n> go run ./cmd/dogego cert wallet-import" -ForegroundColor Yellow
go run ./cmd/dogego cert wallet-import
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if ($IncludePQ) {
    Write-Host "`n> go run ./cmd/dogego cert pq (optional PQ slice)" -ForegroundColor Yellow
    go run ./cmd/dogego cert pq
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($IncludeOperator) {
    Write-Host "`n> go run ./cmd/dogego cert operator (deep Milestone E)" -ForegroundColor Yellow
    go run ./cmd/dogego cert operator
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Write-Host "`nOffline prerequisites passed." -ForegroundColor Green
exit 0
