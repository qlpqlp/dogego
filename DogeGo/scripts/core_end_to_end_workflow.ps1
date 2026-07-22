# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: bundled Core-equivalent operator workflow (live node required).
# Runs health, restart-resume, maintenance, optional Core compare + IBD convergence.
#
#   .\scripts\core_end_to_end_workflow.ps1
#   $env:DOGEGO_CORE_COMPARE = "1"
#   .\scripts\core_end_to_end_workflow.ps1
param(
    [switch]$Json,
    [string]$DataDir,
    [string]$Network = "mainnet"
)
$ErrorActionPreference = "Stop"
$steps = @()
$failed = $false

function Step($Name, $Script, $Args) {
    Write-Host "`n=== $Name ===" -ForegroundColor Cyan
    & $Script @Args
    $ok = ($LASTEXITCODE -eq 0)
    $row = [ordered]@{ name = $Name; ok = $ok; exit = $LASTEXITCODE }
    if ($Json -and $Name -eq "offline_corpus") {
        $row.note = "58 templates via go test TestEvalMempoolCorpus"
    }
    if ($Json -and $Name -eq "bip125_offline") {
        $row.note = "BIP125 rule 2/5 rows: rbf_too_many_conflicts, rbf_new_unconfirmed_input"
    }
    if ($Json -and $Name -eq "mempool_parity") {
        try {
            $jArgs = @("-WebProbe", "-Json")
            if ($env:DOGEGO_RPC_PORT) { $jArgs += @("-RpcPort", [int]$env:DOGEGO_RPC_PORT) }
            $raw = & $Script @jArgs | ConvertFrom-Json
            if ($raw.note) { $row.note = $raw.note }
            if ($null -ne $raw.passed -and $null -ne $raw.total) {
                $row.note = (($row.note, ("stateless " + $raw.passed + "/" + $raw.total)) | Where-Object { $_ }) -join "; "
            }
        } catch { }
    }
    if ($Json -and $Name -eq "restart_resume") {
        try {
            $jArgs = @($Args) + @("-Json")
            $raw = & $Script @jArgs | ConvertFrom-Json
            $noteParts = @()
            if ($null -ne $raw.connect_lag) { $noteParts += ("connect_lag=" + $raw.connect_lag) }
            if ($raw.connect_catch_up_boost) { $noteParts += ("boost=" + $raw.connect_catch_up_boost) }
            if ($noteParts.Count -gt 0) { $row.note = ($noteParts -join " ") }
        } catch { }
    }
    if ($Json -and $Name -eq "wallet_basics") {
        try {
            $jArgs = @($Args) + @("-Json")
            $raw = & $Script @jArgs | ConvertFrom-Json
            $noteParts = @()
            if ($null -ne $raw.wallet_listtransactions_ms) {
                $noteParts += ("listtransactions_40=" + $raw.wallet_listtransactions_ms + "ms")
            }
            if ($raw.wallet_listtransactions_ok -eq $false) { $noteParts += "slow" }
            if ($raw.wallet_tx_hex_ok -eq $true) { $noteParts += "tx_hex_ok" }
            if ($raw.wallet_pq_send_ok -eq $true) {
                $noteParts += "pq_send_ok"
                if ($raw.wallet_pq_tag) { $noteParts += $raw.wallet_pq_tag }
            }
            if ($noteParts.Count -gt 0) { $row.note = ($noteParts -join " ") }
        } catch { }
    }
    $script:steps += $row
    if (-not $ok) { $script:failed = $true }
    return $ok
}

$common = @{}
if ($DataDir) { $common.DataDir = $DataDir }
if ($Network) { $common.Network = $Network }

Step "node_health" "$PSScriptRoot\node_health.ps1" @common
if ($failed) { exit 2 }

Step "restart_resume" "$PSScriptRoot\core_restart_resume_check.ps1" @common
Step "maintenance" "$PSScriptRoot\core_maintenance_workflow.ps1" @common
Step "reindex_check" "$PSScriptRoot\core_reindex_prune_workflow.ps1" @common
Step "offline_corpus" "$PSScriptRoot\core_mempool_corpus_probe.ps1" @()
Step "bip125_offline" "$PSScriptRoot\core_mempool_bip125_offline_probe.ps1" @()
Step "mempool_parity" "$PSScriptRoot\core_mempool_parity_probe.ps1" @("-WebProbe")
if ($failed) { exit 2 }
Step "protocol_lock" "$PSScriptRoot\core_protocol_lock_probe.ps1" @()
if ($Network -eq "reboottestnet" -or $Network -eq "testnet") {
    Step "setup_parity" "$PSScriptRoot\setup_reboottestnet_core_parity.ps1" @()
}
Step "bip152_hb" "$PSScriptRoot\core_bip152_probe.ps1" @()
Step "addrman" "$PSScriptRoot\core_addrman_workflow.ps1" @common

if ($env:DOGEGO_CORE_COMPARE -eq "1") {
    Step "core_parity_probe" "$PSScriptRoot\core_parity_probe.ps1" @()
}
if ($env:DOGEGO_MEMPOOL_PROBE -eq "1") {
    Step "core_mempool_parity" "$PSScriptRoot\core_mempool_parity_probe.ps1" @()
}
if ($env:DOGEGO_IBD_CONVERGE -eq "1") {
    $conv = @{ IntervalSec = 120 }
    if ($DataDir) { $conv.DataDir = $DataDir }
    if ($Network) { $conv.Network = $Network }
    Step "ibd_convergence" "$PSScriptRoot\ibd_convergence_check.ps1" @conv
}

try {
    . "$PSScriptRoot\dogego_rpc.ps1"
    $w = Invoke-DogeGoJsonRpc -Method getwalletinfo -WarmupRetries 2 -WarmupDelaySec 1
    if ($w) {
        Step "wallet_basics" "$PSScriptRoot\core_wallet_workflow.ps1" @common
    }
} catch {
    Write-Host "wallet probe skipped (no wallet or RPC warming)" -ForegroundColor DarkGray
}

$allOk = -not $failed
if ($Json) {
    [ordered]@{ ok = $allOk; steps = $steps } | ConvertTo-Json -Depth 4
} else {
    if ($allOk) {
        Write-Host "`nCore end-to-end workflow passed." -ForegroundColor Green
    } else {
        Write-Host "`nCore end-to-end workflow failed." -ForegroundColor Red
        foreach ($s in $steps) {
            if (-not $s.ok) { Write-Host ("  FAIL: " + $s.name) -ForegroundColor Red }
        }
    }
}

if (-not $allOk) { exit 1 }
exit 0
