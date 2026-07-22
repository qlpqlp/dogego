# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: reboottestnet scripted end-to-end operator runbook (live node + wallet).
# Bundles health, restart-resume, maintenance, BIP152 HB, wallet, offline mempool corpus, stateful mempool probes.
#
#   .\scripts\core_e2e_reboottestnet_runbook.ps1
#   .\scripts\core_e2e_reboottestnet_runbook.ps1 -IncludeReindex
param(
    [switch]$Json,
    [switch]$IncludeReindex,
    [switch]$IncludeCoreCompare,
    [switch]$IncludeRestartWorkflow,
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
    Write-Error "This runbook targets reboottestnet (RelaxedPoW + wallet mine path)."
}

$env:DOGEGO_RPC_PORT = $DogeGoRpcPort

$common = @{ DataDir = $DataDir; Network = $Network }

Step "node_health" "$PSScriptRoot\node_health.ps1" $common
if ($failed) { exit 2 }

Step "restart_resume" "$PSScriptRoot\core_restart_resume_check.ps1" $common
Step "maintenance" "$PSScriptRoot\core_maintenance_workflow.ps1" $common
Step "bip152_hb" "$PSScriptRoot\core_bip152_probe.ps1" @{ RpcPort = [int]$DogeGoRpcPort }
Step "mempool_corpus_offline" "$PSScriptRoot\core_mempool_corpus_probe.ps1" @{}
Step "bip125_offline" "$PSScriptRoot\core_mempool_bip125_offline_probe.ps1" @{}

try {
    . "$PSScriptRoot\dogego_rpc.ps1"
    if ($IncludeCoreCompare) { $env:DOGEGO_CORE_COMPARE = "1" }
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

if ($IncludeReindex) {
    $env:DOGEGO_REBOOTTESTNET_REINDEX = "1"
    Step "reboottestnet_reindex" "$PSScriptRoot\core_reboottestnet_reindex_workflow.ps1" $common
}

if ($IncludeRestartWorkflow) {
    $env:DOGEGO_RESTART_WORKFLOW = "1"
    Step "restart_workflow" "$PSScriptRoot\core_restart_workflow.ps1" $common
}

$allOk = -not $failed
if ($Json) {
    [ordered]@{ ok = $allOk; network = $Network; steps = $steps } | ConvertTo-Json -Depth 4
} else {
    if ($allOk) {
        Write-Host "`nReboottestnet end-to-end runbook passed." -ForegroundColor Green
    } else {
        Write-Host "`nReboottestnet end-to-end runbook failed." -ForegroundColor Red
        foreach ($s in $steps) {
            if (-not $s.ok) { Write-Host ("  FAIL: " + $s.name) -ForegroundColor Red }
        }
    }
}

if (-not $allOk) { exit 1 }
exit 0
