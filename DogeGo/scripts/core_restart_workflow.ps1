# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E: restart workflow - stop/start dogego and verify RPC + resume invariants.
# Disruptive: restarts the local node. Use on dev/testnet or when you accept a brief mainnet pause.
#
#   .\scripts\core_restart_workflow.ps1
#   .\scripts\core_restart_workflow.ps1 -Network testnet -WaitSec 45
param(
    [switch]$Json,
    [switch]$Rebuild,
    [string]$DataDir = "dogedata",
    [string]$Network = "mainnet",
    [int]$WaitSec = 60
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

$before = $null
try {
    $before = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 3 -WarmupDelaySec 2
} catch {
    Write-Host "Node not up before restart - proceeding with start only." -ForegroundColor DarkGray
}

$restartArgs = @("-WaitSec", $WaitSec, "-DataDir", $DataDir, "-Network", $Network)
if ($Rebuild) { $restartArgs += "-Rebuild" }
& "$PSScriptRoot\restart_node.ps1" @restartArgs
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$after = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 10 -WarmupDelaySec 3
$issues = @()
if ($before -and $after) {
    if ([int64]$after.headers -lt [int64]$before.headers - 1000) {
        $issues += "headers_regressed_after_restart"
    }
    if ([int64]$after.blocks -lt [int64]$before.blocks - 64) {
        $issues += "chain_active_regressed_after_restart"
    }
}

& "$PSScriptRoot\core_restart_resume_check.ps1" -Network $Network -DataDir $DataDir
if ($LASTEXITCODE -ne 0) { $issues += "restart_resume_check_failed" }

$ok = ($issues.Count -eq 0)
$report = [ordered]@{
    ok      = $ok
    network = $Network
    before  = $before
    after   = $after
    issues  = @($issues)
}

if ($Json) {
    $report | ConvertTo-Json -Depth 6
} else {
    Write-Host "=== Core restart workflow ===" -ForegroundColor Cyan
    if ($before) { Write-Host ("before: headers={0} blocks={1}" -f $before.headers, $before.blocks) }
    Write-Host ("after:  headers={0} blocks={1} ibd={2}" -f $after.headers, $after.blocks, $after.initialblockdownload)
    foreach ($i in $issues) { Write-Host ("FAIL: " + $i) -ForegroundColor Red }
    if ($ok) { Write-Host "`nRestart workflow passed." -ForegroundColor Green }
    else { Write-Host "`nRestart workflow failed." -ForegroundColor Red }
}

if (-not $ok) { exit 1 }
exit 0
