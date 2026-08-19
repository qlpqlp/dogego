# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E mining cert: getmininginfo + getblocktemplate (Digishield bits, BIP22 longpoll)
# + createauxblock in aux era. Optional Core GBT side-by-side when tips align.
#
#   cd DogeGo
#   .\scripts\core_mining_workflow.ps1
#   .\scripts\core_mining_workflow.ps1 -WebProbe
#   .\scripts\core_mining_workflow.ps1 -DogeGoOnly -Json
param(
    [switch]$DogeGoOnly,
    [switch]$WebProbe,
    [switch]$Json,
    [int]$RpcPort = 0,
    [int]$WebPort = 2013
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

function Emit-Result($Ok, $Note) {
    if ($Json) {
        [ordered]@{ ok = [bool]$Ok; note = $Note } | ConvertTo-Json -Compress
    }
    if (-not $Ok) { exit 1 }
    exit 0
}

if ($WebProbe) {
    try {
        $uri = "http://127.0.0.1:$WebPort/api/core-mining-probe"  # GET /api/core-mining-probe
        $r = Invoke-RestMethod -Uri $uri -TimeoutSec 60
        $note = "gbt=$($r.gbt_fields_ok); mininginfo=$($r.mininginfo_ok); aux_era=$($r.aux_era)"
        if ($r.createaux_ok) { $note += "; createaux=ok" }
        if ($r.createaux_skipped) { $note += "; createaux=skipped" }
        if ($r.core_aligned) { $note += "; core_gbt_aligned" }
        if ($r.skipped) { Emit-Result $false ($r.reason) }
        if (-not $r.ok) { Emit-Result $false $note }
        if (-not $Json) { Write-Host "Web mining probe ok: $note" -ForegroundColor Green }
        Emit-Result $true $note
    } catch {
        if ($Json) { Emit-Result $false $_.Exception.Message }
        throw
    }
}

$issues = @()
$notes = @()

try {
    $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 5 -WarmupDelaySec 2 -RpcPort $RpcPort
} catch {
    $issues += "getblockchaininfo_failed"
    $info = $null
}

$blocks = 0
if ($info) { $blocks = [int64]$info.blocks }

try {
    $mi = Invoke-DogeGoJsonRpc -Method getmininginfo -RpcPort $RpcPort
    foreach ($k in @("blocks", "difficulty", "networkhashps", "pooledtx", "chain")) {
        if (-not ($mi.PSObject.Properties.Name -contains $k)) {
            $issues += "getmininginfo_missing_$k"
        }
    }
} catch {
    $issues += "getmininginfo_failed"
}

try {
    $gbt = Invoke-DogeGoJsonRpc -Method getblocktemplate -Params @(@{}) -RpcPort $RpcPort
} catch {
    $issues += "getblocktemplate_failed"
    $gbt = $null
}

$required = @(
    "capabilities", "version", "rules", "vbrequired", "coinbaseaux", "previousblockhash",
    "bits", "target", "height", "curtime", "mintime", "sigoplimit", "sizelimit", "weightlimit",
    "coinbasevalue", "transactions", "mutable", "noncerange", "longpollid"
)
if ($gbt) {
    foreach ($f in $required) {
        if (-not ($gbt.PSObject.Properties.Name -contains $f)) {
            $issues += "gbt_missing_$f"
        }
    }
    $caps = @($gbt.capabilities)
    if ($caps -notcontains "proposal" -or $caps -notcontains "longpoll") {
        $issues += "gbt_capabilities"
    }
    if (-not $gbt.longpollid) { $issues += "gbt_longpollid_empty" }
    if (-not $gbt.bits) { $issues += "gbt_bits_empty" }
    if (-not $gbt.target) { $issues += "gbt_target_empty" }
    $notes += "gbt_height=$($gbt.height)"
}

# Aux era: mainnet 371337, reboot testnet 158100 â€” use next height from tip.
$net = if ($info -and $info.chain) { [string]$info.chain } else { "main" }
$auxAct = if ($net -match "test") { 158100 } else { 371337 }
$next = $blocks + 1
if ($next -ge $auxAct) {
    try {
        $addr = Invoke-DogeGoJsonRpc -Method getnewaddress -RpcPort $RpcPort
    } catch {
        # Solo mining may lack wallet; use a throwaway validateaddress-safe payout from createaux rejection path.
        $addr = $null
        try {
            $val = Invoke-DogeGoJsonRpc -Method validateaddress -Params @("nZVmAGQDqjTK8ZcefpbObY4M2iEok5RsXN") -RpcPort $RpcPort
            if ($val.isvalid) { $addr = "nZVmAGQDqjTK8ZcefpbObY4M2iEok5RsXN" }
        } catch {}
    }
    if (-not $addr) {
        # Last resort: let createauxblock reject invalid address (still proves RPC exists).
        $addr = "DDummyMiningCertAddress00000000000"
    }
    try {
        $caux = Invoke-DogeGoJsonRpc -Method createauxblock -Params @($addr) -RpcPort $RpcPort
        foreach ($k in @("hash", "chainid", "target", "coinbasevalue")) {
            if (-not ($caux.PSObject.Properties.Name -contains $k)) {
                $issues += "createaux_missing_$k"
            }
        }
        if ([int]$caux.chainid -ne 0x62) { $issues += "createaux_chainid" }
        $notes += "createaux_ok"
    } catch {
        $issues += "createauxblock_failed"
        $notes += $_.Exception.Message
    }
} else {
    $notes += "createaux_skipped_pre_aux_era next=$next activation=$auxAct"
}

$coreAligned = $null
$coreCli = $env:DOGEGO_CORE_CLI
if (-not $coreCli) {
    $coreCli = (Get-Command dogecoin-cli -ErrorAction SilentlyContinue).Source
}
if (-not $DogeGoOnly -and $coreCli -and $gbt -and $info) {
    try {
        $coreInfo = & $coreCli getblockchaininfo 2>$null | ConvertFrom-Json
        if ([int64]$coreInfo.blocks -eq $blocks) {
            $coreGbt = & $coreCli getblocktemplate "{}" 2>$null | ConvertFrom-Json
            $aligned = $true
            foreach ($k in @("previousblockhash", "bits", "target", "height")) {
                if ("$($gbt.$k)" -ne "$($coreGbt.$k)") {
                    $aligned = $false
                    $issues += "core_gbt_drift_$k"
                }
            }
            $coreAligned = $aligned
            if ($aligned) { $notes += "core_gbt_aligned" }
        } else {
            $notes += "core_tip_mismatch dogego=$blocks core=$($coreInfo.blocks)"
        }
    } catch {
        $notes += "core_unreachable_for_mining_compare"
    }
}

$ok = ($issues.Count -eq 0)
$note = ($notes -join "; ")
if ($issues.Count -gt 0) { $note = ($issues -join ",") + " | " + $note }
if (-not $Json) {
    if ($ok) { Write-Host "Mining workflow ok: $note" -ForegroundColor Green }
    else { Write-Host "Mining workflow FAIL: $note" -ForegroundColor Red }
}
Emit-Result $ok $note
