# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Stateless testmempoolaccept parity (Milestone D/E).
# Uses consensus/testdata/mempool_parity_rpc.json (32 stateless rows).
#
#   .\scripts\core_mempool_parity_probe.ps1                 # DogeGo + Core side-by-side (Core required)
#   .\scripts\core_mempool_parity_probe.ps1 -DogeGoOnly      # DogeGo vs corpus only (E2E default)
#   .\scripts\core_mempool_parity_probe.ps1 -WebProbe       # GET /api/mempool/parity-probe on web UI
#   .\scripts\core_mempool_parity_probe.ps1 -DogeGoOnly -Json
param(
    [switch]$DogeGoOnly,
    [switch]$WebProbe,
    [switch]$Json,
    [int]$RpcPort = 22557
)
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

function Emit-Result($Ok, $Passed, $Total, $Note) {
    if ($Json) {
        [ordered]@{
            ok     = [bool]$Ok
            passed = [int]$Passed
            total  = [int]$Total
            note   = $Note
        } | ConvertTo-Json -Compress
    }
    if (-not $Ok) { exit 1 }
    exit 0
}

if ($WebProbe) {
    try {
        $uri = "http://127.0.0.1:$RpcPort/api/mempool/parity-probe"
        $r = Invoke-RestMethod -Uri $uri -TimeoutSec 60
        $note = "stateless $($r.passed)/$($r.total)"
        if ($r.offline_corpus -and $r.offline_corpus.total -gt 0) {
            $note += "; corpus $($r.offline_corpus.passed)/$($r.offline_corpus.total)"
        }
        if ($r.skipped) {
            Emit-Result $false 0 0 ($r.reason)
        }
        if (-not $r.ok) {
            Emit-Result $false ($r.passed) ($r.total) $note
        }
        if ($r.core_configured -and $r.core_available -and $r.core_aligned -eq $false) {
            Emit-Result $false ($r.passed) ($r.total) ($note + "; core drift")
        }
        if (-not $Json) {
            Write-Host "Web mempool parity ok: $note" -ForegroundColor Green
        }
        Emit-Result $true ($r.passed) ($r.total) $note
    } catch {
        if ($Json) {
            Emit-Result $false 0 0 $_.Exception.Message
        }
        throw
    }
}

$fixturePath = Join-Path $DogeGo "consensus\testdata\mempool_parity_rpc.json"
if (-not (Test-Path $fixturePath)) {
    Write-Host "Missing $fixturePath - run: `$env:UPDATE_MEMPOOL_PARITY_RPC='1'; go test ./consensus -run TestUpdateMempoolParityRPCFixture -count=1" -ForegroundColor Red
    exit 1
}

. "$PSScriptRoot\dogego_rpc.ps1"

$coreCli = $env:DOGEGO_CORE_CLI
if (-not $coreCli) {
    $coreCli = (Get-Command dogecoin-cli -ErrorAction SilentlyContinue).Source
}
if (-not $DogeGoOnly -and -not $coreCli) {
    Write-Host "dogecoin-cli not found - use -DogeGoOnly or -WebProbe for solo DogeGo gate." -ForegroundColor Yellow
    exit 0
}

$dgPort = if ($env:DOGEGO_RPC_PORT) { $env:DOGEGO_RPC_PORT } else { "$RpcPort" }
$dgUser = if ($env:DOGEGO_RPC_USER) { $env:DOGEGO_RPC_USER } else { "dogego" }
$dgPass = if ($env:DOGEGO_RPC_PASS) { $env:DOGEGO_RPC_PASS } else { "dogego" }
$corePort = if ($env:DOGEGO_CORE_RPC_PORT) { $env:DOGEGO_CORE_RPC_PORT } else { $dgPort }
$coreUser = if ($env:DOGEGO_CORE_RPC_USER) { $env:DOGEGO_CORE_RPC_USER } else { $dgUser }
$corePass = if ($env:DOGEGO_CORE_RPC_PASS) { $env:DOGEGO_CORE_RPC_PASS } else { $dgPass }

function Invoke-TestMempoolAccept($isCore, $hex) {
    if ($isCore) {
        $param = '["' + $hex + '"]'
        $args = @("-rpcport=$corePort", "testmempoolaccept", $param)
        if ($coreUser) { $args = @("-rpcuser=$coreUser", "-rpcpassword=$corePass") + $args }
        $out = & $coreCli @args 2>&1
        if ($LASTEXITCODE -ne 0) { throw $out }
        $parsed = $out | ConvertFrom-Json
    } else {
        $parsed = Invoke-DogeGoJsonRpc -Method testmempoolaccept -Params @(@($hex)) -RpcPort ([int]$dgPort) -RpcUser $dgUser -RpcPassword $dgPass
    }
    if ($parsed -is [System.Array]) { return $parsed[0] }
    return $parsed
}

function Reason-Matches($got, $want) {
    if (-not $want) { return $true }
    if (-not $got) { return $false }
    return ($got -eq $want) -or ($got.StartsWith($want))
}

$rows = Get-Content $fixturePath -Raw | ConvertFrom-Json
$title = if ($DogeGoOnly) { "DogeGo testmempoolaccept (stateless corpus)" } else { "Core vs DogeGo testmempoolaccept (stateless corpus)" }
if (-not $Json) {
    Write-Host "=== $title ===" -ForegroundColor Cyan
}
$failed = 0
$passed = 0
foreach ($row in $rows) {
    if (-not $Json) {
        Write-Host ("`n[{0}] template={1}" -f $row.name, $row.template) -ForegroundColor Yellow
    }
    $dgRow = Invoke-TestMempoolAccept $false $row.hex
    if (-not $Json) {
        Write-Host ("  DogeGo: allowed={0} reject={1}" -f $dgRow.allowed, $dgRow.'reject-reason')
    }

    if ($dgRow.allowed -ne $row.want_accept) {
        if (-not $Json) { Write-Host "  FAIL DogeGo allowed mismatch" -ForegroundColor Red }
        $failed++
        continue
    }
    if (-not $row.want_accept -and -not (Reason-Matches $dgRow.'reject-reason' $row.want_reject_reason)) {
        if (-not $Json) { Write-Host ("  FAIL DogeGo reject-reason want {0}" -f $row.want_reject_reason) -ForegroundColor Red }
        $failed++
        continue
    }

    if (-not $DogeGoOnly) {
        $coreRow = Invoke-TestMempoolAccept $true $row.hex
        if (-not $Json) {
            Write-Host ("  Core:   allowed={0} reject={1}" -f $coreRow.allowed, $coreRow.'reject-reason')
        }
        if ($coreRow.allowed -ne $row.want_accept) {
            if (-not $Json) { Write-Host "  WARN Core allowed differs from corpus (may be version/policy drift)" -ForegroundColor Yellow }
            $passed++
            continue
        }
        if (-not $row.want_accept -and -not (Reason-Matches $coreRow.'reject-reason' $row.want_reject_reason)) {
            if (-not $Json) { Write-Host ("  WARN Core reject-reason {0} vs want {1}" -f $coreRow.'reject-reason', $row.want_reject_reason) -ForegroundColor Yellow }
            $passed++
            continue
        }
    }

    $passed++
    if (-not $Json) { Write-Host "  OK" -ForegroundColor Green }
}

$total = @($rows).Count
if ($failed -gt 0) {
    if (-not $Json) {
        Write-Host "`nMempool parity probe failed ($failed row(s))." -ForegroundColor Red
    }
    Emit-Result $false $passed $total "failed=$failed"
}
if (-not $Json) {
    Write-Host "`nMempool parity probe passed ($passed/$total)." -ForegroundColor Green
}
Emit-Result $true $passed $total "stateless $passed/$total"
