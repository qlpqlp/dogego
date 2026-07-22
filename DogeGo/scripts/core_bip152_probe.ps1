# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# BIP152 v1 high-bandwidth compact block probe (Milestone E).
# Checks DogeGo getpeerinfo for bip152_hb_to / bip152_hb_from and HB negotiation when caught up.
#
#   cd DogeGo
#   .\scripts\core_bip152_probe.ps1
#   .\scripts\core_bip152_probe.ps1 -RpcPort 44556
param(
    [switch]$Json,
    [int]$RpcPort = 0
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

$issues = @()
$notes = @()

try {
    $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 5 -WarmupDelaySec 2 -RpcPort $RpcPort
} catch {
    $issues += "getblockchaininfo_failed"
    $info = $null
}

$ibd = $false
if ($info -and $info.initialblockdownload -eq $true) { $ibd = $true }

try {
    $peers = Invoke-DogeGoJsonRpc -Method getpeerinfo -RpcPort $RpcPort
} catch {
    $issues += "getpeerinfo_failed"
    $peers = @()
}

if ($null -eq $peers) { $peers = @() }
if ($peers -isnot [System.Array]) { $peers = @($peers) }

$schemaOK = $true
$hbTo = 0
$hbFrom = 0
foreach ($p in $peers) {
    if ($null -eq $p.PSObject.Properties["bip152_hb_to"] -or $null -eq $p.PSObject.Properties["bip152_hb_from"]) {
        $schemaOK = $false
    }
    if ($p.bip152_hb_to -eq $true) { $hbTo++ }
    if ($p.bip152_hb_from -eq $true) { $hbFrom++ }
}

if (-not $schemaOK) { $issues += "getpeerinfo_missing_bip152_fields" }
if ($peers.Count -gt 0 -and $hbTo -eq 0 -and $hbFrom -eq 0 -and -not $ibd) {
    $issues += "no_bip152_hb_negotiated"
}
if ($peers.Count -eq 0) {
    $notes += "no_peers_connected"
} elseif ($hbTo -gt 0 -or $hbFrom -gt 0) {
    $notes += "hb_to=$hbTo hb_from=$hbFrom peers=$($peers.Count)"
} elseif ($ibd) {
    $notes += "ibd: HB may be deferred on ephemeral header-sync links"
}

$ok = ($issues.Count -eq 0) -and $schemaOK

$coreCli = $env:DOGEGO_CORE_CLI
if (-not $coreCli) {
    $coreCli = (Get-Command dogecoin-cli -ErrorAction SilentlyContinue).Source
}
if (-not $coreCli) {
    $coreDefault = "C:\Program Files\Dogecoin\daemon\dogecoin-cli.exe"
    if (Test-Path $coreDefault) { $coreCli = $coreDefault }
}
$coreAvailable = $false
$corePeerCount = 0
$coreHbTo = 0
$coreHbFrom = 0
$coreHbNegotiated = $false
if ($coreCli) {
    $corePort = if ($env:DOGEGO_CORE_RPC_PORT) { $env:DOGEGO_CORE_RPC_PORT } else { if ($RpcPort -gt 0) { $RpcPort } else { "22557" } }
    $coreUser = if ($env:DOGEGO_CORE_RPC_USER) { $env:DOGEGO_CORE_RPC_USER } else { $env:DOGEGO_RPC_USER }
    $corePass = if ($env:DOGEGO_CORE_RPC_PASS) { $env:DOGEGO_CORE_RPC_PASS } else { $env:DOGEGO_RPC_PASS }
    $coreArgs = @("-rpcport=$corePort", "getpeerinfo")
    if ($coreUser) { $coreArgs = @("-rpcuser=$coreUser", "-rpcpassword=$corePass") + $coreArgs }
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $coreOut = & $coreCli @coreArgs 2>&1
    $coreCode = $LASTEXITCODE
    $ErrorActionPreference = $prevEap
    if ($coreCode -eq 0) {
        $coreAvailable = $true
        $corePeers = $coreOut | ConvertFrom-Json
        if ($null -eq $corePeers) { $corePeers = @() }
        if ($corePeers -isnot [System.Array]) { $corePeers = @($corePeers) }
        $corePeerCount = $corePeers.Count
        foreach ($cp in $corePeers) {
            if ($cp.bip152_hb_to -eq $true) { $coreHbTo++ }
            if ($cp.bip152_hb_from -eq $true) { $coreHbFrom++ }
        }
        $coreHbNegotiated = ($coreHbTo -gt 0) -or ($coreHbFrom -gt 0)
        if (-not $ibd -and $peers.Count -gt 0 -and $corePeerCount -gt 0) {
            if (($hbTo -gt 0 -or $hbFrom -gt 0) -and $coreHbNegotiated) {
                $notes += "hb_negotiated_dogego_and_core"
            } elseif ($hbTo -eq 0 -and $hbFrom -eq 0 -and -not $coreHbNegotiated) {
                $notes += "hb_not_negotiated_either_node"
            } elseif (($hbTo -gt 0 -or $hbFrom -gt 0) -ne $coreHbNegotiated) {
                $notes += "hb_negotiation_asymmetric_vs_core"
            }
        }
    } else {
        $notes += ("core_unreachable: " + (($coreOut | Out-String).Trim()))
    }
} else {
    $notes += "core_compare_optional"
}

$cmpctRelay = @{}
$cmpctRelaySchemaOK = $false
if ($info) {
    $cmpctRelaySchemaOK = $true
    foreach ($k in @(
        "dogego_cmpct_in", "dogego_cmpct_mempool_hit", "dogego_cmpct_getblocktxn_out",
        "dogego_cmpct_blocktxn_in", "dogego_cmpct_reconstruct_ok", "dogego_cmpct_reconstruct_fail",
        "dogego_cmpct_announced_out", "dogego_cmpct_served_getdata", "dogego_cmpct_fallback_full_block",
        "dogego_cmpct_blocktxn_served", "dogego_cmpct_reconstruct_fallback_getdata"
    )) {
        if ($null -eq $info.PSObject.Properties[$k]) {
            $cmpctRelaySchemaOK = $false
        }
    }
    if ($peers.Count -gt 0 -and -not $ibd -and -not $cmpctRelaySchemaOK) {
        $issues += "cmpct_relay_counters_missing"
    }
    foreach ($k in @(
        "dogego_cmpct_in", "dogego_cmpct_mempool_hit", "dogego_cmpct_getblocktxn_out",
        "dogego_cmpct_blocktxn_in", "dogego_cmpct_reconstruct_ok", "dogego_cmpct_reconstruct_fail",
        "dogego_cmpct_announced_out", "dogego_cmpct_served_getdata", "dogego_cmpct_fallback_full_block",
        "dogego_cmpct_blocktxn_served", "dogego_cmpct_reconstruct_fallback_getdata"
    )) {
        if ($null -ne $info.PSObject.Properties[$k]) {
            $cmpctRelay[$k] = [int64]$info.$k
        }
    }
    if (-not $ibd -and $peers.Count -gt 0 -and ($hbTo -gt 0 -or $hbFrom -gt 0)) {
        $active = ($cmpctRelay["dogego_cmpct_reconstruct_ok"] -gt 0) -or
            ($cmpctRelay["dogego_cmpct_announced_out"] -gt 0) -or
            ($cmpctRelay["dogego_cmpct_served_getdata"] -gt 0)
        if ($active) { $notes += "cmpct_relay_active" }
        elseif ($cmpctRelay.Count -gt 0) { $notes += "cmpct_relay_idle" }
        if (($cmpctRelay["dogego_cmpct_fallback_full_block"] -gt 0) -or
            ($cmpctRelay["dogego_cmpct_reconstruct_fallback_getdata"] -gt 0)) {
            $notes += "cmpct_full_block_fallback_seen"
        }
    }
}

$out = [ordered]@{
    ok          = $ok
    ibd         = $ibd
    peer_count  = $peers.Count
    hb_to_peers = $hbTo
    hb_from_peers = $hbFrom
    schema_ok   = $schemaOK
    core_available = $coreAvailable
    core_peer_count = $corePeerCount
    core_hb_to_peers = $coreHbTo
    core_hb_from_peers = $coreHbFrom
    core_hb_negotiated = $coreHbNegotiated
    issues      = $issues
    notes       = $notes
    cmpct_relay = $cmpctRelay
    cmpct_relay_schema_ok = $cmpctRelaySchemaOK
    hint        = "BIP152 v1: sendcmpct HB negotiate; cmpctblock relay when caught up. Web: GET /api/core-bip152-probe"
}

if ($Json) {
    $out | ConvertTo-Json -Depth 6
} else {
    Write-Host "=== BIP152 compact block probe ===" -ForegroundColor Cyan
    Write-Host ("peers={0} hb_to={1} hb_from={2} ibd={3} schema_ok={4}" -f $peers.Count, $hbTo, $hbFrom, $ibd, $schemaOK)
    if ($coreAvailable) {
        Write-Host ("core peers={0} hb_to={1} hb_from={2} hb_negotiated={3}" -f $corePeerCount, $coreHbTo, $coreHbFrom, $coreHbNegotiated) -ForegroundColor DarkGray
    }
    if ($cmpctRelay.Count -gt 0) {
        Write-Host ("cmpct in={0} reconstruct_ok={1} announced={2}" -f $cmpctRelay["dogego_cmpct_in"], $cmpctRelay["dogego_cmpct_reconstruct_ok"], $cmpctRelay["dogego_cmpct_announced_out"]) -ForegroundColor DarkGray
    }
    foreach ($n in $notes) { Write-Host ("  note: " + $n) -ForegroundColor DarkGray }
    foreach ($i in $issues) { Write-Host ("  issue: " + $i) -ForegroundColor Red }
    if ($ok) {
        Write-Host "BIP152 probe passed." -ForegroundColor Green
        exit 0
    }
    Write-Host "BIP152 probe failed." -ForegroundColor Red
    exit 1
}

if (-not $ok) { exit 1 }
