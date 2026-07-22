# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: operator workflow certification (offline + optional live mainnet probes).
#
#   .\scripts\core_operator_workflow_cert.ps1
#   $env:DOGEGO_IBD_SOAK = "1"
#   .\scripts\core_operator_workflow_cert.ps1
#   $env:DOGEGO_IBD_SOAK = "1"
#   $env:DOGEGO_IBD_CONVERGE = "1"
#   .\scripts\core_operator_workflow_cert.ps1
#   $env:DOGEGO_CORE_COMPARE = "1"
#   .\scripts\core_operator_workflow_cert.ps1
# Or run all flags at once:
#   .\scripts\core_operator_runbook_full.ps1
#   .\scripts\core_operator_runbook_full.ps1 -Mainnet -AllowMainnet -CoreCompare
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

Write-Host "=== Core operator workflow certification ===" -ForegroundColor Cyan

if ($env:DOGEGO_CORRUPTION_SOAK -eq "1") {
    Write-Host "`n[offline] Corruption / kill recovery tests" -ForegroundColor Yellow
    & "$PSScriptRoot\corruption_soak_cert.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

& "$PSScriptRoot\operator_workflow_cert.ps1"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "`n[offline] Milestone D mempool corpus (58 templates)" -ForegroundColor Yellow
& "$PSScriptRoot\core_mempool_corpus_probe.ps1"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "`n[offline] Milestone D BIP125 rule 2/5 (subset gate)" -ForegroundColor Yellow
& "$PSScriptRoot\core_mempool_bip125_offline_probe.ps1"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if ($env:DOGEGO_IBD_SOAK -ne "1") {
    Write-Host "`nLive probes skipped. Enable with:" -ForegroundColor DarkGray
    Write-Host "  `$env:DOGEGO_IBD_SOAK = `"1`"; .\scripts\core_operator_workflow_cert.ps1" -ForegroundColor DarkGray
    Write-Host "Optional: DOGEGO_CORE_COMPARE=1 runs core_side_by_side_full.ps1; DOGEGO_RESTART_WORKFLOW=1 runs disruptive restart cert" -ForegroundColor DarkGray
    Write-Host "Optional: DOGEGO_IBD_CONVERGE=1 DOGEGO_ADDRMAN_PROBE=1 DOGEGO_MEMPOOL_PROBE=1 DOGEGO_BIP152_PROBE=1 DOGEGO_BIP152_LIVE_SOAK=1 DOGEGO_RESTART_RESUME=1 DOGEGO_RESTART_CONNECT_CHECK=1 DOGEGO_MAINTENANCE_PROBE=1 DOGEGO_CORRUPTION_SOAK=1 DOGEGO_CORRUPTION_INJECT=1 DOGEGO_CORRUPTION_INJECT_SOAK=1 DOGEGO_REINDEX_PROBE=1 DOGEGO_TIMED_SOAK=1 DOGEGO_IBD_LIVE_SOAK=1" -ForegroundColor DarkGray
    Write-Host "All-in-one: .\scripts\core_operator_runbook_full.ps1  (-OfflineOnly for CI)" -ForegroundColor DarkGray
    Write-Host "`nOffline Core operator workflow certification passed." -ForegroundColor Green
    exit 0
}

& "$PSScriptRoot\ibd_soak_cert.ps1"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "`n[protocol-lock] Solo deployment sanity (no Core required)" -ForegroundColor Yellow
& "$PSScriptRoot\core_protocol_lock_probe.ps1"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if ($env:DOGEGO_IBD_CONVERGE -eq "1") {
    Write-Host "`n[convergence] Body/IBD forward progress window" -ForegroundColor Yellow
    $convArgs = @()
    if ($env:DOGEGO_CONVERGE_INTERVAL_SEC) { $convArgs += "-IntervalSec", [int]$env:DOGEGO_CONVERGE_INTERVAL_SEC }
    if ($env:DOGEGO_CONVERGE_MIN_PROBE) { $convArgs += "-MinRawProbeAdvance", [int64]$env:DOGEGO_CONVERGE_MIN_PROBE }
    & "$PSScriptRoot\ibd_convergence_check.ps1" @convArgs
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_ADDRMAN_PROBE -eq "1") {
    Write-Host "`n[addrman] getaddrmaninfo bucket snapshot" -ForegroundColor Yellow
    & "$PSScriptRoot\core_addrman_workflow.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_RESTART_RESUME -eq "1") {
    Write-Host "`n[restart-resume] Checkpoint vs contiguous + assist pool" -ForegroundColor Yellow
    & "$PSScriptRoot\core_restart_resume_check.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    if ($env:DOGEGO_RESTART_CONNECT_CHECK -eq "1") {
        Write-Host "`n[restart-connect] Post-restart chainActive catch-up" -ForegroundColor Yellow
        $ccArgs = @{}
        if ($env:DOGEGO_RESTART_CONNECT_MAX_LAG) { $ccArgs.MaxLag = [int64]$env:DOGEGO_RESTART_CONNECT_MAX_LAG }
        if ($env:DOGEGO_RESTART_CONNECT_TIMEOUT) { $ccArgs.TimeoutSec = [int]$env:DOGEGO_RESTART_CONNECT_TIMEOUT }
        & "$PSScriptRoot\core_restart_connect_check.ps1" @ccArgs
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
}

if ($env:DOGEGO_MAINTENANCE_PROBE -eq "1") {
    Write-Host "`n[maintenance] verifychain / getindexinfo / getchaintxstats" -ForegroundColor Yellow
    & "$PSScriptRoot\core_maintenance_workflow.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_REINDEX_PROBE -eq "1") {
    Write-Host "`n[reindex] getindexinfo + maintenance RPC presence" -ForegroundColor Yellow
    & "$PSScriptRoot\core_reindex_prune_workflow.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    if ($env:DOGEGO_NETWORK -eq "mainnet" -or $env:DOGEGO_CORE_COMPARE -eq "1") {
        & "$PSScriptRoot\core_mainnet_reindex_compare.ps1"
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
}

if ($env:DOGEGO_BIP152_PROBE -eq "1") {
    Write-Host "`n[bip152] getpeerinfo HB negotiate" -ForegroundColor Yellow
    & "$PSScriptRoot\core_bip152_probe.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_BIP152_LIVE_SOAK -eq "1") {
    Write-Host "`n[bip152-live-soak] timed HB + cmpct relay window" -ForegroundColor Yellow
    $bipSoakArgs = @{}
    if ($env:DOGEGO_BIP152_SOAK_MIN) { $bipSoakArgs.DurationMin = [int]$env:DOGEGO_BIP152_SOAK_MIN }
    if ($env:DOGEGO_BIP152_SOAK_INTERVAL) { $bipSoakArgs.IntervalSec = [int]$env:DOGEGO_BIP152_SOAK_INTERVAL }
    if ($env:DOGEGO_BIP152_SOAK_REQUIRE_RELAY -eq "1") { $bipSoakArgs.RequireRelayActivity = $true }
    & "$PSScriptRoot\bip152_live_soak_gate.ps1" @bipSoakArgs
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_REBOOTTESTNET_REINDEX -eq "1") {
    Write-Host "`n[reindex] reboottestnet reindextx workflow" -ForegroundColor Yellow
    & "$PSScriptRoot\core_reboottestnet_reindex_workflow.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_TIMED_SOAK -eq "1") {
    Write-Host "`n[timed-soak] Repeated health/convergence window" -ForegroundColor Yellow
    $soakArgs = @()
    if ($env:DOGEGO_TIMED_SOAK_MIN) { $soakArgs += "-DurationMin", [int]$env:DOGEGO_TIMED_SOAK_MIN }
    if ($env:DOGEGO_TIMED_SOAK_INTERVAL) { $soakArgs += "-IntervalSec", [int]$env:DOGEGO_TIMED_SOAK_INTERVAL }
    if ($env:DOGEGO_TIMED_SOAK_REQUIRE_CONVERGE -eq "1") { $soakArgs += "-RequireConvergence" }
    if ($env:DOGEGO_TIMED_SOAK_AUTO_RESTART -eq "1") { $soakArgs += "-AutoRestartOnStaleLock" }
    & "$PSScriptRoot\ibd_timed_soak.ps1" @soakArgs
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_IBD_LIVE_SOAK -eq "1") {
    Write-Host "`n[live-soak] Mainnet IBD gate (timed + convergence + auto-resume)" -ForegroundColor Yellow
    $liveArgs = @{}
    if ($env:DOGEGO_TIMED_SOAK_MIN) { $liveArgs.DurationMin = [int]$env:DOGEGO_TIMED_SOAK_MIN }
    if ($env:DOGEGO_TIMED_SOAK_INTERVAL) { $liveArgs.IntervalSec = [int]$env:DOGEGO_TIMED_SOAK_INTERVAL }
    & "$PSScriptRoot\ibd_live_soak_gate.ps1" @liveArgs
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_EXTENDED_SOAK -eq "1") {
    Write-Host "`n[extended-soak] Timed IBD + corruption inject cycle" -ForegroundColor Yellow
    $extArgs = @{}
    if ($env:DOGEGO_TIMED_SOAK_MIN) { $extArgs.DurationMin = [int]$env:DOGEGO_TIMED_SOAK_MIN }
    if ($env:DOGEGO_TIMED_SOAK_INTERVAL) { $extArgs.IntervalSec = [int]$env:DOGEGO_TIMED_SOAK_INTERVAL }
    & "$PSScriptRoot\extended_operator_soak.ps1" @extArgs
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_CORE_COMPARE -eq "1") {
    Write-Host "`n[Core compare] Side-by-side full probe bundle" -ForegroundColor Yellow
    & "$PSScriptRoot\core_side_by_side_full.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_MEMPOOL_PROBE -eq "1") {
    Write-Host "`n[mempool] testmempoolaccept side-by-side vs Core" -ForegroundColor Yellow
    & "$PSScriptRoot\core_mempool_parity_probe.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_MEMPOOL_STATEFUL_PROBE -eq "1") {
    Write-Host "`n[mempool] stateful parity probe (reboottestnet, -Scenario all)" -ForegroundColor Yellow
    & "$PSScriptRoot\mempool_stateful_parity_reboottestnet.ps1" -Scenario all
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_MEMPOOL_STATEFUL_CORE_GATE -eq "1") {
    Write-Host "`n[mempool] stateful Core gate (compare required)" -ForegroundColor Yellow
    & "$PSScriptRoot\mempool_stateful_core_gate.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_REBOOTTESTNET_CORE_GATE -eq "1") {
    Write-Host "`n[Core] reboottestnet aligned gate (24/24 stateful compare)" -ForegroundColor Yellow
    & "$PSScriptRoot\core_reboottestnet_core_aligned_gate.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_REINDEX_DISRUPTIVE -eq "1") {
    Write-Host "`n[reindex] disruptive reindex/prune workflow" -ForegroundColor Yellow
    & "$PSScriptRoot\core_reindex_prune_disruptive_workflow.ps1" -IncludeCoreCompare
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_RECOVERY_PROBE -eq "1") {
    Write-Host "`n[recovery] dogego_recoverheaders RPC probe" -ForegroundColor Yellow
    & "$PSScriptRoot\core_recovery_workflow.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_CORRUPTION_INJECT -eq "1") {
    Write-Host "`n[corruption-inject] Live headers tail truncate + restart (disruptive; reboottestnet default)" -ForegroundColor Yellow
    & "$PSScriptRoot\corruption_inject_live.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_CORRUPTION_INJECT_SOAK -eq "1") {
    Write-Host "`n[corruption-inject-soak] Live headers/raw/filter inject cycle (disruptive; reboottestnet default)" -ForegroundColor Yellow
    & "$PSScriptRoot\corruption_inject_soak.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_CORRUPTION_TIMED_LOOP -eq "1") {
    Write-Host "`n[corruption-timed-loop] Repeated inject soak with verifychain (disruptive)" -ForegroundColor Yellow
    $loopArgs = @{}
    if ($env:DOGEGO_CORRUPTION_LOOP_MIN) { $loopArgs.DurationMin = [int]$env:DOGEGO_CORRUPTION_LOOP_MIN }
    if ($env:DOGEGO_CORRUPTION_LOOP_INTERVAL) { $loopArgs.IntervalMin = [int]$env:DOGEGO_CORRUPTION_LOOP_INTERVAL }
    if ($env:DOGEGO_CORRUPTION_CYCLES) { $loopArgs.CorruptionCycles = [int]$env:DOGEGO_CORRUPTION_CYCLES }
    if ($env:DOGEGO_CORRUPTION_TIMED_MINI -eq "1") {
        & "$PSScriptRoot\corruption_timed_loop_mini.ps1" @loopArgs
    } else {
        & "$PSScriptRoot\corruption_timed_loop.ps1" @loopArgs
    }
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_CORRUPTION_EXTENDED_MINI -eq "1") {
    Write-Host "`n[corruption-extended-mini] Timed health + corruption loop (raw/index/filter)" -ForegroundColor Yellow
    & "$PSScriptRoot\corruption_extended_cert_mini.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_CORRUPTION_LONG_SOAK -eq "1") {
    Write-Host "`n[corruption-long-soak] Extended timed corruption gate (disruptive)" -ForegroundColor Yellow
    & "$PSScriptRoot\corruption_long_soak_gate.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_CI_LIVE_GATE -eq "1") {
    Write-Host "`n[ci-live] Reboottestnet live CI gate bundle" -ForegroundColor Yellow
    & "$PSScriptRoot\ci_live_reboottestnet_gate.ps1" -IncludeCoreAlignedGate -IncludeCorruptionMini
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_SCHEDULED_WEEKLY_LIVE -eq "1" -or $env:DOGEGO_WEEKLY_LIVE_GATE -eq "1") {
    Write-Host "`n[weekly-live] Scheduled weekly live CI bundle" -ForegroundColor Yellow
    & "$PSScriptRoot\ci_scheduled_weekly_live.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_RUNNER_PREFLIGHT -eq "1") {
    Write-Host "`n[runner-preflight] dogego-live readiness" -ForegroundColor Yellow
    $pfArgs = @{}
    if ($env:DOGEGO_CORE_COMPARE_REQUIRED -eq "1") { $pfArgs.RequireCore = $true }
    & "$PSScriptRoot\ci_runner_preflight.ps1" @pfArgs
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($env:DOGEGO_RESTART_WORKFLOW -eq "1") {
    Write-Host "`n[restart] Stop/start + resume invariants (disruptive)" -ForegroundColor Yellow
    & "$PSScriptRoot\core_restart_workflow.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Write-Host "`nCore operator workflow certification passed." -ForegroundColor Green
exit 0
