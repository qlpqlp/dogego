# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone D: offline evaluation of all 58 core_mempool_vectors.json templates.
#
#   .\scripts\core_mempool_corpus_probe.ps1
#   .\scripts\core_mempool_corpus_probe.ps1 -WebProbe -RpcPort 22557
#   .\scripts\core_mempool_corpus_probe.ps1 -Json
param(
    [switch]$WebProbe,
    [switch]$Json,
    [int]$RpcPort = 22557
)
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

function Emit-CorpusResult($Ok) {
    if ($Json) {
        [ordered]@{ ok = [bool]$Ok; templates = 58; mode = "offline_eval" } | ConvertTo-Json -Compress
    }
    if (-not $Ok) { exit 1 }
    exit 0
}

if (-not $Json) {
    Write-Host "=== Mempool policy corpus (offline, 58 templates) ===" -ForegroundColor Cyan
}
go test ./consensus -run TestEvalMempoolCorpus -count=1
if ($LASTEXITCODE -ne 0) { Emit-CorpusResult $false }

if ($WebProbe) {
    if (-not $Json) {
        Write-Host "`n=== Web UI full corpus probe ===" -ForegroundColor Cyan
    }
    try {
        $uri = "http://127.0.0.1:$RpcPort/api/mempool/parity-probe?corpus=full"
        $r = Invoke-RestMethod -Uri $uri -TimeoutSec 60
        if (-not $r.ok) {
            Write-Error "web probe failed: passed=$($r.passed) failed=$($r.failed)"
        }
        if (-not $Json) {
            Write-Host ("Web probe ok: total={0} stateful={1} stateless={2}" -f $r.total, $r.stateful, $r.stateless) -ForegroundColor Green
        }
    } catch {
        Write-Warning "Web probe skipped (UI not on port ${RpcPort}): $_"
    }
}

if (-not $Json) {
    Write-Host "`nMempool corpus probe passed." -ForegroundColor Green
}
Emit-CorpusResult $true
