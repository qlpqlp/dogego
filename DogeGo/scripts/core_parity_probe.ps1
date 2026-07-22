# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Compare DogeGo vs Dogecoin Core RPC on the same network (Milestone E side-by-side probe).
# Requires both nodes running. Skips gracefully when Core CLI is absent.
#
#   cd DogeGo
#   .\scripts\core_parity_probe.ps1
#
# Environment (optional):
#   $env:DOGEGO_RPC_PORT = "22557"   # DogeGo default; Core keeps :22555
#   $env:DOGEGO_RPC_USER = "dogego"
#   $env:DOGEGO_RPC_PASS = "dogego"
#   $env:DOGEGO_CORE_CLI = "C:\Program Files\Dogecoin\daemon\dogecoin-cli.exe"
#   $env:DOGEGO_CORE_RPC_PORT = "22556"   # when Core uses a different port than DogeGo
#   $env:DOGEGO_CORE_RPC_USER / DOGEGO_CORE_RPC_PASS
#   $env:DOGEGO_PARITY_MAX_HEADER_DELTA = "100"
#   $env:DOGEGO_PARITY_MAX_BLOCK_DELTA = "500"
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

. "$PSScriptRoot\dogego_rpc.ps1"

$coreCli = $env:DOGEGO_CORE_CLI
if (-not $coreCli) {
    $coreCli = (Get-Command dogecoin-cli -ErrorAction SilentlyContinue).Source
}
if (-not $coreCli) {
    $coreDefault = "C:\Program Files\Dogecoin\daemon\dogecoin-cli.exe"
    if (Test-Path $coreDefault) { $coreCli = $coreDefault }
}
if (-not $coreCli) {
    Write-Host "dogecoin-cli not found - set DOGEGO_CORE_CLI or add Core to PATH. Skipping side-by-side probe." -ForegroundColor Yellow
    exit 0
}

$dgPort = if ($env:DOGEGO_RPC_PORT) { $env:DOGEGO_RPC_PORT } else { "22557" }
$dgUser = if ($env:DOGEGO_RPC_USER) { $env:DOGEGO_RPC_USER } else { "dogego" }
$dgPass = if ($env:DOGEGO_RPC_PASS) { $env:DOGEGO_RPC_PASS } else { "dogego" }
$corePort = if ($env:DOGEGO_CORE_RPC_PORT) { $env:DOGEGO_CORE_RPC_PORT } else { $dgPort }
$coreUser = if ($env:DOGEGO_CORE_RPC_USER) { $env:DOGEGO_CORE_RPC_USER } else { $dgUser }
$corePass = if ($env:DOGEGO_CORE_RPC_PASS) { $env:DOGEGO_CORE_RPC_PASS } else { $dgPass }
$maxHeaderDelta = 100
if ($env:DOGEGO_PARITY_MAX_HEADER_DELTA) { $maxHeaderDelta = [int]$env:DOGEGO_PARITY_MAX_HEADER_DELTA }
$maxBlockDelta = 500
if ($env:DOGEGO_PARITY_MAX_BLOCK_DELTA) { $maxBlockDelta = [int]$env:DOGEGO_PARITY_MAX_BLOCK_DELTA }

function Invoke-DogeGoRpc($method) {
    $rpcParams = @{ RpcPort = [int]$dgPort; WarmupRetries = 15; WarmupDelaySec = 4 }
    if ($dgUser) { $rpcParams.RpcUser = $dgUser }
    if ($dgPass) { $rpcParams.RpcPassword = $dgPass }
    return Invoke-DogeGoJsonRpc -Method $method @rpcParams
}

function Invoke-CoreRpc($method) {
    $args = @("-rpcport=$corePort", $method)
    if ($coreUser) { $args = @("-rpcuser=$coreUser", "-rpcpassword=$corePass") + $args }
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $out = & $coreCli @args 2>&1
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prevEap
    if ($code -ne 0) {
        $text = ($out | Out-String).Trim()
        if ($text -match "couldn't connect|Connection refused|EOF reached|timeout") {
            throw "core_unreachable: $text"
        }
        throw "Core $method failed: $text"
    }
    return $out | ConvertFrom-Json
}

Write-Host "=== Core vs DogeGo RPC parity probe ===" -ForegroundColor Cyan
Write-Host ("DogeGo RPC port: {0}  Core: {1}:{2}" -f $dgPort, $coreCli, $corePort)

$dg = Invoke-DogeGoRpc "getblockchaininfo"
try {
    $core = Invoke-CoreRpc "getblockchaininfo"
} catch {
    if ($_.Exception.Message -match "core_unreachable") {
        Write-Host "Core RPC unreachable on port $corePort - DogeGo-only health check." -ForegroundColor Yellow
        Write-Host ("DogeGo headers={0} blocks={1} ibd={2} chain={3}" -f $dg.headers, $dg.blocks, $dg.initialblockdownload, $dg.chain)
        if ($dg.PSObject.Properties.Name -contains "dogego_contiguous_raw_height") {
            Write-Host ("DogeGo contiguous_raw_height={0}" -f $dg.dogego_contiguous_raw_height)
        }
        Write-Host "Side-by-side compare skipped (start Dogecoin Core on RPC port $corePort to enable)." -ForegroundColor Yellow
        exit 0
    }
    throw
}

Write-Host ("`nchain: DogeGo={0}  Core={1}" -f $dg.chain, $core.chain)
if ($dg.chain -ne $core.chain) {
    Write-Host "FAIL: chain name mismatch" -ForegroundColor Red
    exit 1
}

Write-Host ("headers: DogeGo={0}  Core={1}  delta={2}" -f $dg.headers, $core.headers, [Math]::Abs($dg.headers - $core.headers))
Write-Host ("blocks:  DogeGo={0}  Core={1}  delta={2}" -f $dg.blocks, $core.blocks, [Math]::Abs($dg.blocks - $core.blocks))
Write-Host ("ibd:     DogeGo={0}  Core={1}" -f $dg.initialblockdownload, $core.initialblockdownload)
if ($dg.bestblockhash -and $core.bestblockhash) {
    Write-Host ("bestblockhash: DogeGo={0}  Core={1}" -f $dg.bestblockhash, $core.bestblockhash)
}
if ($dg.chainwork -and $core.chainwork) {
    $cwMatch = ($dg.chainwork -eq $core.chainwork)
    Write-Host ("chainwork: DogeGo={0}  Core={1} match={2}" -f $dg.chainwork, $core.chainwork, $cwMatch)
    if (-not $cwMatch -and $dg.bestblockhash -eq $core.bestblockhash) {
        Write-Host "FAIL: chainwork mismatch at shared tip" -ForegroundColor Red
        exit 1
    }
}
if (($null -ne $dg.mediantime) -and ($null -ne $core.mediantime)) {
    Write-Host ("mediantime: DogeGo={0}  Core={1}" -f $dg.mediantime, $core.mediantime)
}
if ((-not $dg.initialblockdownload) -and (-not $core.initialblockdownload)) {
    if (($null -ne $dg.verificationprogress) -and ($null -ne $core.verificationprogress)) {
        $vpDelta = [Math]::Abs([double]$dg.verificationprogress - [double]$core.verificationprogress)
        $vpMatch = ($vpDelta -le 0.05) -or (($dg.verificationprogress -ge 0.999) -and ($core.verificationprogress -ge 0.999))
        Write-Host ("verificationprogress: DogeGo={0}  Core={1}  delta={2} match={3}" -f $dg.verificationprogress, $core.verificationprogress, $vpDelta, $vpMatch)
        if (-not $vpMatch) {
            Write-Host "WARN: verification progress diverged while caught up" -ForegroundColor Yellow
        }
    }
}

$headerDelta = [Math]::Abs([int64]$dg.headers - [int64]$core.headers)
$blockDelta = [Math]::Abs([int64]$dg.blocks - [int64]$core.blocks)
if ($headerDelta -gt $maxHeaderDelta) {
    Write-Host "FAIL: header tip delta $headerDelta exceeds $maxHeaderDelta" -ForegroundColor Red
    exit 1
}
if ($blockDelta -gt $maxBlockDelta) {
    Write-Host "WARN: block height delta $blockDelta exceeds $maxBlockDelta (DogeGo connect may lag stored bodies during IBD)" -ForegroundColor Yellow
}

if ($dg.PSObject.Properties.Name -contains "dogego_genesis_missing" -and $dg.dogego_genesis_missing -eq $true) {
    Write-Host "WARN: DogeGo dogego_genesis_missing=true" -ForegroundColor Yellow
}
if ($dg.PSObject.Properties.Name -contains "dogego_contiguous_raw_height") {
    Write-Host ("DogeGo contiguous_raw_height={0}" -f $dg.dogego_contiguous_raw_height)
}

try {
    $dgVerifyOut = Invoke-DogeGoJsonRpc -Method verifychain -Params @(4, 0) -RpcPort ([int]$dgPort) -RpcUser $dgUser -RpcPassword $dgPass
} catch {
    Write-Host "FAIL: DogeGo verifychain: $_" -ForegroundColor Red
    exit 1
}
$coreVerifyArgs = @("-rpcport=$corePort", "verifychain", "4", "0")
if ($coreUser) { $coreVerifyArgs = @("-rpcuser=$coreUser", "-rpcpassword=$corePass") + $coreVerifyArgs }
$coreVerifyOut = & $coreCli @coreVerifyArgs 2>&1
$coreVerifyOk = ($LASTEXITCODE -eq 0)
Write-Host ("`nverifychain 4 0: DogeGo=$dgVerifyOut  Core=$coreVerifyOut")
if ($dgVerifyOut -ne $true -and "$dgVerifyOut" -notmatch "true") {
    Write-Host "FAIL: DogeGo verifychain did not return true" -ForegroundColor Red
    exit 1
}
if (-not $coreVerifyOk) {
    Write-Host "WARN: Core verifychain failed (Core may still be syncing)" -ForegroundColor Yellow
}

if (-not $dg.initialblockdownload -and -not $core.initialblockdownload) {
    try {
        $dgUtxo = Invoke-DogeGoRpc "gettxoutsetinfo"
        $coreUtxo = Invoke-CoreRpc "gettxoutsetinfo"
        Write-Host ("`ngettxoutsetinfo height: DogeGo={0} Core={1}" -f $dgUtxo.height, $coreUtxo.height)
        if ($dgUtxo.height -ne $coreUtxo.height) {
            Write-Host "WARN: UTXO set height differs (DogeGo connect may lag Core on same datadir age)" -ForegroundColor Yellow
        }
        if ($dgUtxo.hash_serialized -and $coreUtxo.hash_serialized -and $dgUtxo.hash_serialized -ne $coreUtxo.hash_serialized) {
            Write-Host "NOTE: hash_serialized differs - expected during IBD catch-up or different connect tip" -ForegroundColor DarkGray
        }
    } catch {
        Write-Host "gettxoutsetinfo skipped: $_" -ForegroundColor DarkGray
    }

    try {
        $dgDep = Invoke-DogeGoRpc "getdeploymentinfo"
        $coreDep = Invoke-CoreRpc "getdeploymentinfo"
        $dgNames = @($dgDep.deployments.PSObject.Properties.Name)
        $coreNames = @($coreDep.deployments.PSObject.Properties.Name)
        $allNames = ($dgNames + $coreNames | Select-Object -Unique | Sort-Object)
        $lockOk = $true
        foreach ($name in $allNames) {
            $dgDeploy = $dgDep.deployments.$name
            $coreDeploy = $coreDep.deployments.$name
            $dgActive = $false
            $coreActive = $false
            if ($dgDeploy) { $dgActive = [bool]$dgDeploy.active }
            if ($coreDeploy) { $coreActive = [bool]$coreDeploy.active }
            $match = ($dgActive -eq $coreActive)
            if (-not $match) { $lockOk = $false }
            Write-Host ("deployment.{0}.active: DogeGo={1} Core={2} match={3}" -f $name, $dgActive, $coreActive, $match)
            if ($dgDeploy -and $coreDeploy -and $dgDeploy.bip9 -and $coreDeploy.bip9) {
                $dgStatus = $dgDeploy.bip9.status
                $coreStatus = $coreDeploy.bip9.status
                if ($dgStatus -and $coreStatus -and ($dgStatus -ne $coreStatus)) {
                    $lockOk = $false
                    Write-Host ("  deployment.{0}.status: DogeGo={1} Core={2} MISMATCH" -f $name, $dgStatus, $coreStatus) -ForegroundColor Red
                }
                if (($null -ne $dgDeploy.bip9.bit) -and ($null -ne $coreDeploy.bip9.bit) -and ([int]$dgDeploy.bip9.bit -ne [int]$coreDeploy.bip9.bit)) {
                    $lockOk = $false
                    Write-Host ("  deployment.{0}.bit: DogeGo={1} Core={2} MISMATCH" -f $name, $dgDeploy.bip9.bit, $coreDeploy.bip9.bit) -ForegroundColor Red
                }
            }
        }
        if (-not $lockOk) {
            Write-Host "FAIL: deployment protocol-lock mismatch (consensus fork risk)" -ForegroundColor Red
            exit 1
        }
        Write-Host "deployment.protocol_lock: OK" -ForegroundColor Green

        if ($dg.softforks -and $core.softforks) {
            $sfNames = @()
            foreach ($sf in @($dg.softforks)) { if ($sf.id) { $sfNames += $sf.id } }
            foreach ($sf in @($core.softforks)) { if ($sf.id -and ($sfNames -notcontains $sf.id)) { $sfNames += $sf.id } }
            foreach ($name in ($sfNames | Sort-Object -Unique)) {
                $dgReject = $false
                $coreReject = $false
                foreach ($sf in @($dg.softforks)) {
                    if ($sf.id -eq $name -and $sf.reject) { $dgReject = [bool]$sf.reject.status }
                }
                foreach ($sf in @($core.softforks)) {
                    if ($sf.id -eq $name -and $sf.reject) { $coreReject = [bool]$sf.reject.status }
                }
                $match = ($dgReject -eq $coreReject)
                if (-not $match) { $lockOk = $false }
                Write-Host ("softfork.{0}.reject: DogeGo={1} Core={2} match={3}" -f $name, $dgReject, $coreReject, $match)
            }
        }
        if ($dg.bip9_softforks -and $core.bip9_softforks) {
            $bip9Names = @($dg.bip9_softforks.PSObject.Properties.Name + $core.bip9_softforks.PSObject.Properties.Name | Select-Object -Unique | Sort-Object)
            foreach ($name in $bip9Names) {
                $dgStatus = $dg.bip9_softforks.$name.status
                $coreStatus = $core.bip9_softforks.$name.status
                if ($dgStatus -and $coreStatus -and ($dgStatus -ne $coreStatus)) {
                    $lockOk = $false
                    Write-Host ("bip9_softfork.{0}.status: DogeGo={1} Core={2} MISMATCH" -f $name, $dgStatus, $coreStatus) -ForegroundColor Red
                }
            }
        }
        if (-not $lockOk) {
            Write-Host "FAIL: softfork protocol-lock mismatch" -ForegroundColor Red
            exit 1
        }
    } catch {
        Write-Host "getdeploymentinfo skipped: $_" -ForegroundColor DarkGray
    }
}

Write-Host "`nCore parity probe passed (within configured deltas)." -ForegroundColor Green

try {
    $dgTips = Invoke-DogeGoRpc "getchaintips"
    $active = @($dgTips | Where-Object { $_.status -eq "active" })
    Write-Host ("getchaintips: {0} tip(s), active={1}" -f @($dgTips).Count, @($active).Count)
    if (@($active).Count -ne 1) {
        Write-Host "WARN: expected exactly one active chain tip" -ForegroundColor Yellow
    }
} catch {
    Write-Host "getchaintips skipped: $_" -ForegroundColor DarkGray
}

if ($dg.PSObject.Properties.Name -contains "dogego_filter_index_lag") {
    Write-Host ("filter_index_lag={0}" -f $dg.dogego_filter_index_lag)
}
if ($dg.initialblockdownload -eq $true -and $dg.PSObject.Properties.Name -contains "dogego_raw_sync") {
    $rs = $dg.dogego_raw_sync
    if ($rs.assist_peer_pool -ne $null) {
        Write-Host ("IBD assist_peer_pool={0} sessions={1}" -f $rs.assist_peer_pool, $rs.assist_active_sessions)
        if ([int]$rs.assist_peer_pool -eq 0 -and $dg.headers -gt 10000) {
            Write-Host "WARN: assist pool empty during IBD (Core-style body download may stall)" -ForegroundColor Yellow
        }
    }
}

try {
    $zmq = Invoke-DogeGoRpc "getzmqnotifications"
    Write-Host ("getzmqnotifications: {0} endpoint(s)" -f @($zmq).Count)
} catch {
    Write-Host "getzmqnotifications skipped: $_" -ForegroundColor DarkGray
}

exit 0
