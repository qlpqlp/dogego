# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: full reboottestnet operator runbook (IBD, restart, reindex/prune, wallet, recovery, mempool).
#
#   .\scripts\core_e2e_full_runbook.ps1
#   .\scripts\core_e2e_full_runbook.ps1 -IncludeDisruptive -IncludeCoreCompare
param(
    [switch]$Json,
    [switch]$IncludeDisruptive,
    [switch]$IncludeCoreCompare,
    [switch]$IncludeCorruptionMini,
    [string]$DataDir = "dogedata",
    [string]$Network = "reboottestnet",
    [string]$DogeGoRpcPort = "44556"
)
$ErrorActionPreference = "Stop"
$steps = @()
$failed = $false

function Step {
    param([string]$Name, [string]$Script, [hashtable]$Args = @{})
    Write-Host "`n=== $Name ===" -ForegroundColor Cyan
    & $Script @Args
    $ok = ($LASTEXITCODE -eq 0)
    $script:steps += [ordered]@{ name = $Name; ok = $ok; exit = $LASTEXITCODE }
    if (-not $ok) { $script:failed = $true }
    return $ok
}

if ($Network -ne "reboottestnet") {
    Write-Error "Full E2E runbook targets reboottestnet."
}

$env:DOGEGO_RPC_PORT = $DogeGoRpcPort

$common = @{ DataDir = $DataDir; Network = $Network }

Write-Host "=== Reboottestnet full E2E runbook ===" -ForegroundColor Cyan

Step "offline_operator_cert" "$PSScriptRoot\operator_workflow_cert.ps1" @{}
if ($failed) { exit 2 }

Step "node_health" "$PSScriptRoot\node_health.ps1" $common
if ($failed) { exit 2 }

Step "ibd_convergence" "$PSScriptRoot\ibd_convergence_check.ps1" @{
    Network           = $Network
    DataDir           = $DataDir
    IntervalSec       = 60
    MinBlocksAdvance  = 0
    MinContiguousAdvance = 0
}
Step "restart_resume" "$PSScriptRoot\core_restart_resume_check.ps1" $common
Step "maintenance" "$PSScriptRoot\core_maintenance_workflow.ps1" $common
Step "reindex_prune_check" "$PSScriptRoot\core_reindex_prune_workflow.ps1" @{ Network = $Network }
Step "bip152_hb" "$PSScriptRoot\core_bip152_probe.ps1" @{ RpcPort = [int]$DogeGoRpcPort }
Step "recovery_probe" "$PSScriptRoot\core_recovery_workflow.ps1" @{ Network = $Network }
Step "mempool_corpus_offline" "$PSScriptRoot\core_mempool_corpus_probe.ps1" @{}
Step "bip125_offline" "$PSScriptRoot\core_mempool_bip125_offline_probe.ps1" @{}

if ($IncludeCoreCompare) {
    $env:DOGEGO_CORE_COMPARE = "1"
    $env:DOGEGO_CORE_COMPARE_REQUIRED = "1"
}

try {
    . "$PSScriptRoot\dogego_rpc.ps1"
    $w = Invoke-DogeGoJsonRpc -Method getwalletinfo -RpcPort ([int]$DogeGoRpcPort) -WarmupRetries 3 -WarmupDelaySec 2
    if ($w) {
        Step "wallet_basics" "$PSScriptRoot\core_wallet_workflow.ps1" $common
        Step "mempool_stateful_all" "$PSScriptRoot\mempool_stateful_parity_reboottestnet.ps1" @{
            Scenario      = "all"
            Network       = $Network
            DogeGoRpcPort = $DogeGoRpcPort
        }
    }
} catch {
    Write-Host "wallet/stateful probes skipped: $_" -ForegroundColor DarkGray
    $script:steps += [ordered]@{ name = "wallet_basics"; ok = $false; skipped = $true }
}

if ($IncludeDisruptive) {
    $env:DOGEGO_REBOOTTESTNET_REINDEX = "1"
    Step "reboottestnet_reindex" "$PSScriptRoot\core_reboottestnet_reindex_workflow.ps1" $common
    Step "reindex_prune_disruptive" "$PSScriptRoot\core_reindex_prune_disruptive_workflow.ps1" (@{
        Network       = $Network
        DataDir       = $DataDir
        DogeGoRpcPort = $DogeGoRpcPort
    } + $(if ($IncludeCoreCompare) { @{ IncludeCoreCompare = $true } } else { @{} }))
    $env:DOGEGO_RESTART_WORKFLOW = "1"
    Step "restart_workflow" "$PSScriptRoot\core_restart_workflow.ps1" $common
}

if ($IncludeCorruptionMini) {
    Step "corruption_extended_mini" "$PSScriptRoot\corruption_extended_cert_mini.ps1" @{ Network = $Network; DataDir = $DataDir }
}

$allOk = -not $failed
if ($Json) {
    [ordered]@{ ok = $allOk; network = $Network; steps = $steps } | ConvertTo-Json -Depth 4
} else {
    if ($allOk) {
        Write-Host "`nReboottestnet full E2E runbook passed." -ForegroundColor Green
    } else {
        Write-Host "`nReboottestnet full E2E runbook failed." -ForegroundColor Red
        foreach ($s in $steps) {
            if (-not $s.ok) { Write-Host ("  FAIL: " + $s.name) -ForegroundColor Red }
        }
    }
}

if (-not $allOk) { exit 1 }
exit 0
