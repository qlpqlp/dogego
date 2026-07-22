# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Pair two DogeGo nodes on the same home LAN via addnode (DNS seeds do not discover LAN PCs).
#
# Usage:
#   .\lan_peer_pair.ps1 -OtherHost 192.168.1.214
#   .\lan_peer_pair.ps1 -OtherHost 192.168.1.214:44556 -Mutual
#   .\lan_peer_pair.ps1 -ShowHint

param(
    [string]$OtherHost = "",
    [switch]$Mutual,
    [switch]$ShowHint,
    [switch]$OneTry
)

$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

function Get-DogeGoP2PPort {
    param($Conf)
    $net = if ($Conf.network) { $Conf.network } else { "testnet" }
    switch ($net) {
        { $_ -in @("main", "mainnet") } { return 22556 }
        default { return 44556 }
    }
}

function Get-DogeGoLanIPv4 {
    $addrs = @()
    Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue | ForEach-Object {
        $ip = $_.IPAddress
        if ($ip -match '^(10\.|192\.168\.|172\.(1[6-9]|2[0-9]|3[0-1])\.)' -and $ip -ne "127.0.0.1") {
            $addrs += $ip
        }
    }
    return $addrs | Select-Object -Unique
}

function Format-DogeGoPeerTarget {
    param([string]$HostOrTarget, [int]$DefaultPort)
    $s = $HostOrTarget.Trim()
    if (-not $s) { return "" }
    if ($s -match ':') { return $s }
    return "${s}:$DefaultPort"
}

$conf = Read-DogeGoConfig
$port = Get-DogeGoP2PPort -Conf $conf
$locals = Get-DogeGoLanIPv4
$share = if ($locals.Count) { "$( $locals[0] ):$port" } else { "(no private IPv4 detected)" }

if ($ShowHint) {
    Write-Host "Network: $(if ($conf.network) { $conf.network } else { 'testnet' })"
    Write-Host "P2P port: $port"
    Write-Host "Share this target with the other PC: $share"
    if ($locals.Count -gt 1) {
        Write-Host "All LAN IPv4:"
        $locals | ForEach-Object { Write-Host "  ${_}:$port" }
    }
    Write-Host ""
    Write-Host "On the other PC run: addnode `"$share`" add"
    Write-Host "Or use Settings -> P2P -> Pair with another PC on your LAN"
    exit 0
}

if (-not $OtherHost) {
    Write-Host "Usage: .\lan_peer_pair.ps1 -OtherHost OTHER_LAN_IP [-Mutual] [-OneTry] [-ShowHint]"
    Write-Host "Share target on this PC: $share"
    exit 1
}

$target = Format-DogeGoPeerTarget -HostOrTarget $OtherHost -DefaultPort $port
$cmd = if ($OneTry) { "onetry" } else { "add" }

Write-Host "addnode $target $cmd on this node..."
Invoke-DogeGoJsonRpc -Method addnode -Params @($target, $cmd) | Out-Null
Write-Host "OK"

if ($Mutual) {
    Write-Host ""
    Write-Host "Mutual pairing: on the OTHER PC run:"
    Write-Host "  .\lan_peer_pair.ps1 -OtherHost $share"
    Write-Host "Or Console -> addnode -> [""$share"", ""add""]"
}

$peers = Invoke-DogeGoJsonRpc -Method getconnectioncount
Write-Host "connection count: $peers"
