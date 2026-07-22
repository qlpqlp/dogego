# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Shared DogeGo JSON-RPC over HTTP (Core-style POST). Dot-source from operator scripts:
#   . "$PSScriptRoot\dogego_rpc.ps1"
#   $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo
#
# Overrides (optional): DOGEGO_RPC_URI, DOGEGO_RPC_USER, DOGEGO_RPC_PASS, DOGEGO_DATADIR, DOGECOINCONF

function Get-DogeGoRepoRoot {
    if ($script:DogeGoRepoRoot) { return $script:DogeGoRepoRoot }
    $script:DogeGoRepoRoot = Split-Path -Parent $PSScriptRoot
    return $script:DogeGoRepoRoot
}

function Get-DogeGoConfigFile {
    if ($env:DOGECOINCONF -and (Test-Path $env:DOGECOINCONF)) {
        return $env:DOGECOINCONF
    }
    $root = Get-DogeGoRepoRoot
    $candidates = @(
        (Join-Path $env:APPDATA "DogeGo\dogecoinconf.json")
        (Join-Path $env:APPDATA "dogego\dogecoinconf.json")
        (Join-Path $root "dogecoinconf.json")
        (Join-Path $root "dogedata\dogecoinconf.json")
    )
    foreach ($p in $candidates) {
        if ($p -and (Test-Path $p)) { return $p }
    }
    return $null
}

function Read-DogeGoConfig {
    $path = Get-DogeGoConfigFile
    if (-not $path) { return @{} }
    try {
        return (Get-Content -Raw -Path $path | ConvertFrom-Json)
    } catch {
        return @{}
    }
}

function Get-DogeGoChainDataDir {
    param($Conf)
    $base = $env:DOGEGO_DATADIR
    if (-not $base -and $Conf.datadir) { $base = $Conf.datadir }
    if (-not $base) {
        $base = Join-Path (Get-DogeGoRepoRoot) "dogedata"
    }
    if (-not [System.IO.Path]::IsPathRooted($base)) {
        $base = Join-Path (Get-DogeGoRepoRoot) $base
    }
    $net = if ($Conf.network) { $Conf.network } else { "mainnet" }
    switch ($net) {
        "testnet" { return (Join-Path $base "testnet") }
        "reboottestnet" { return (Join-Path $base "reboottestnet") }
        default { return (Join-Path $base "mainnet") }
    }
}

function Get-DogeGoDefaultRpcPort {
    param($Conf = $null)
    if (-not $Conf) { $Conf = Read-DogeGoConfig }
    $net = if ($Conf.network) { [string]$Conf.network } else { "mainnet" }
    switch ($net.ToLowerInvariant()) {
        "testnet" { return 44555 }
        "reboottestnet" { return 44556 }
        default { return 22557 }
    }
}

function Get-DogeGoEffectiveRpcPort {
    param(
        $Conf,
        [int]$ExplicitPort = 0
    )
    if (-not $Conf) { $Conf = Read-DogeGoConfig }
    if ($ExplicitPort -gt 0) { return $ExplicitPort }
    $default = Get-DogeGoDefaultRpcPort -Conf $Conf
    if ($Conf.rpc) {
        $rpc = [string]$Conf.rpc
        if ($rpc -match ':(\d+)\s*$') { return [int]$Matches[1] }
        if ($rpc -match '^\d+$') { return [int]$rpc }
    }
    if ($env:DOGEGO_RPC_PORT) {
        $envPort = [int]$env:DOGEGO_RPC_PORT
        # Legacy env from Core side-by-side docs pointed at :22555; DogeGo full nodes default :22557.
        if ($envPort -eq 22555 -and $default -eq 22557) { return 22557 }
        return $envPort
    }
    return $default
}

function Resolve-DogeGoRpcUri {
    param(
        $Conf,
        [string]$RpcHost,
        [int]$RpcPort = 0
    )
    if ($env:DOGEGO_RPC_URI) { return $env:DOGEGO_RPC_URI.TrimEnd('/') + "/" }
    if ($Conf.rpc -and $RpcPort -le 0) {
        $rpc = [string]$Conf.rpc
        if ($rpc -match '^https?://') { return $rpc.TrimEnd('/') + "/" }
        if ($rpc -match ':') { return "http://$rpc/" }
        return "http://127.0.0.1:$rpc/"
    }
    $hostName = if ($RpcHost) { $RpcHost } else { "127.0.0.1" }
    $port = Get-DogeGoEffectiveRpcPort -Conf $Conf -ExplicitPort $RpcPort
    return "http://${hostName}:$port/"
}

function Get-DogeGoRpcCredentials {
    param($Conf)
    $user = if ($env:DOGEGO_RPC_USER) { $env:DOGEGO_RPC_USER } else { $null }
    $pass = if ($env:DOGEGO_RPC_PASS) { $env:DOGEGO_RPC_PASS } else { $null }
    if ($user -and $pass) { return @{ User = $user; Password = $pass } }

    $chainDir = Get-DogeGoChainDataDir -Conf $Conf
    $cookiePath = Join-Path $chainDir ".cookie"
    if (($Conf.rpc_cookie -eq $true) -or (Test-Path $cookiePath)) {
        if (Test-Path $cookiePath) {
            $line = (Get-Content -Raw -Path $cookiePath).Trim()
            $idx = $line.IndexOf(':')
            if ($idx -gt 0) {
                return @{
                    User     = $line.Substring(0, $idx)
                    Password = $line.Substring($idx + 1)
                }
            }
        }
    }
    if ($Conf.rpc_user -and $Conf.rpc_password) {
        return @{ User = [string]$Conf.rpc_user; Password = [string]$Conf.rpc_password }
    }
    if ($user -or $pass) {
        return @{ User = $(if ($user) { $user } else { "dogego" }); Password = $(if ($pass) { $pass } else { "dogego" }) }
    }
    return @{ User = "dogego"; Password = "dogego" }
}

function Invoke-DogeGoJsonRpc {
    param(
        [Parameter(Mandatory)][string]$Method,
        [object[]]$Params = @(),
        [string]$RpcUser,
        [string]$RpcPassword,
        [string]$RpcHost,
        [int]$RpcPort = 0,
        [int]$WarmupRetries = 0,
        [int]$WarmupDelaySec = 3,
        [int]$TimeoutSec = 60
    )
    $conf = Read-DogeGoConfig
    $uri = Resolve-DogeGoRpcUri -Conf $conf -RpcHost $RpcHost -RpcPort $RpcPort
    $creds = Get-DogeGoRpcCredentials -Conf $conf
    if ($RpcUser) { $creds.User = $RpcUser }
    if ($RpcPassword) { $creds.Password = $RpcPassword }

    $bodyObj = @{
        jsonrpc = "1.0"
        id      = 1
        method  = $Method
        params  = @($Params)
    }
    $body = $bodyObj | ConvertTo-Json -Depth 20 -Compress
    $pair = "{0}:{1}" -f $creds.User, $creds.Password
    $b64 = [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes($pair))
    $headers = @{
        Authorization  = "Basic $b64"
        "Content-Type" = "application/json"
    }
    $attempt = 0
    while ($true) {
        try {
            $raw = Invoke-RestMethod -Uri $uri -Method Post -Headers $headers -Body $body -TimeoutSec $TimeoutSec
        } catch {
            throw "DogeGo RPC $Method @ $uri failed: $_"
        }
        if (-not $raw.error) {
            return $raw.result
        }
        $code = 0
        if ($null -ne $raw.error.code) { $code = [int]$raw.error.code }
        $msg = if ($raw.error.message) { $raw.error.message } else { ($raw.error | ConvertTo-Json -Compress) }
        if ($code -eq -28 -and $attempt -lt $WarmupRetries) {
            $attempt++
            Start-Sleep -Seconds $WarmupDelaySec
            continue
        }
        if ($code -eq -28) {
            throw "DogeGo RPC $Method warming up (-28): $msg. Port is open; retry in a minute or use Web UI http://127.0.0.1:2013/api/summary."
        }
        throw "DogeGo RPC $Method error: $msg"
    }
}

function Get-DogeGoWebUIUrl {
    $conf = Read-DogeGoConfig
    if ($conf.webui) { return "http://$($conf.webui)" }
    return "http://127.0.0.1:2013"
}

function Get-DogeGoWebSummary {
    param([string]$WebUI)
    $base = if ($WebUI) { $WebUI.TrimEnd('/') } else { (Get-DogeGoWebUIUrl).TrimEnd('/') }
    $r = Invoke-WebRequest -Uri ($base + "/api/summary") -UseBasicParsing -TimeoutSec 8
    return ($r.Content | ConvertFrom-Json)
}

function Test-DogeGoAutostartGate {
    param($Conf)
    if (-not $Conf) { $Conf = Read-DogeGoConfig }
    $want = ($null -ne $Conf.autostart) -and ([string]$Conf.autostart).ToLowerInvariant() -eq "login"
    if (-not $want) {
        return [pscustomobject]@{
            ok      = $true
            skipped = $true
            want    = $false
            note    = "autostart_not_configured"
        }
    }
    try {
        $base = (Get-DogeGoWebUIUrl).TrimEnd('/')
        $st = Invoke-RestMethod -Uri ($base + "/api/autostart") -TimeoutSec 8
        if ($st.status -and $st.status.installed) {
            return [pscustomobject]@{
                ok        = $true
                want      = $true
                installed = $true
                method    = [string]$st.status.method
                detail    = [string]$st.status.detail
            }
        }
        if ($st.status -and $st.status.supported -eq $false) {
            return [pscustomobject]@{
                ok      = $true
                want    = $true
                warning = "autostart_login_unsupported_platform"
                detail  = [string]$st.status.detail
            }
        }
    } catch { }
    if ($IsWindows -or ($env:OS -match "Windows")) {
        schtasks /Query /TN "DogeGo Node" /FO LIST 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) {
            return [pscustomobject]@{
                ok        = $true
                want      = $true
                installed = $true
                method    = "Task Scheduler (ONLOGON)"
            }
        }
    }
    return [pscustomobject]@{
        ok    = $false
        want  = $true
        issue = "autostart_login_not_registered"
    }
}

function Test-DogeGoRpcConfigured {
    $conf = Read-DogeGoConfig
    return [bool]($conf.rpc -and ([string]$conf.rpc).Trim().Length -gt 0)
}

function Get-DogeGoChainDir {
    param(
        [string]$DataDir,
        [string]$Network = "mainnet"
    )
    $conf = Read-DogeGoConfig
    $base = $DataDir
    if (-not $base) {
        $base = $env:DOGEGO_DATADIR
        if (-not $base -and $conf.datadir) { $base = $conf.datadir }
        if (-not $base) { $base = Join-Path (Get-DogeGoRepoRoot) "dogedata" }
    }
    if (-not [System.IO.Path]::IsPathRooted($base)) {
        $base = Join-Path (Get-DogeGoRepoRoot) $base
    }
    $net = if ($Network) { $Network } elseif ($conf.network) { $conf.network } else { "mainnet" }
    switch ($net) {
        "testnet" { return (Join-Path $base "testnet") }
        "reboottestnet" { return (Join-Path $base "reboottestnet") }
        default { return (Join-Path $base "mainnet") }
    }
}

# Remove-DogeGoStaleProcessLock deletes .dogego-process.lock when its pid is not running (crash recovery).
function Remove-DogeGoStaleProcessLock {
    param(
        [string]$DataDir,
        [string]$Network = "mainnet"
    )
    $chainDir = Get-DogeGoChainDir -DataDir $DataDir -Network $Network
    $lockPath = Join-Path $chainDir ".dogego-process.lock"
    if (-not (Test-Path $lockPath)) { return $false }
    $lockText = Get-Content $lockPath -Raw -ErrorAction SilentlyContinue
    if ($lockText -match 'pid=(\d+)') {
        $lockPid = [int]$Matches[1]
        if (Get-Process -Id $lockPid -ErrorAction SilentlyContinue) {
            return $false
        }
    }
    Remove-Item $lockPath -Force -ErrorAction SilentlyContinue
    return $true
}

function Get-DogeGoDiskSyncSnapshot {
    param(
        [string]$DataDir,
        [string]$Network = "mainnet"
    )
    $chainDir = Get-DogeGoChainDir -DataDir $DataDir -Network $Network
    $hdrPath = Join-Path $chainDir "headers_sync.json"
    $rawPath = Join-Path $chainDir "rawblocks_sync.json"
    $snap = [ordered]@{
        ChainDir   = $chainDir
        HeaderTip  = $null
        RawProbe   = $null
        HeaderLayout = $null
        Lag        = $null
        BodyPct    = $null
    }
    if (Test-Path $hdrPath) {
        $h = Get-Content -Raw $hdrPath | ConvertFrom-Json
        $snap.HeaderTip = [int64]$h.tip_height
        $snap.HeaderLayout = [string]$h.layout
        $snap.HeaderSyncMtime = (Get-Item $hdrPath).LastWriteTime
    }
    if (Test-Path $rawPath) {
        $r = Get-Content -Raw $rawPath | ConvertFrom-Json
        $snap.RawProbe = [int64]$r.next_probe_height
        $snap.RawSyncMtime = (Get-Item $rawPath).LastWriteTime
    }
    if ($null -ne $snap.HeaderTip -and $null -ne $snap.RawProbe -and $snap.HeaderTip -gt 0) {
        $snap.Lag = $snap.HeaderTip - $snap.RawProbe
        $snap.BodyPct = [math]::Round(100.0 * $snap.RawProbe / $snap.HeaderTip, 3)
    }
    return [pscustomobject]$snap
}

# Connect lag from getblockchaininfo (prefers dogego_connect_lag when present on newer binaries).
function Get-DogeGoRpcConnectLag {
    param($Info)
    if ($null -eq $Info) { return $null }
    if ($Info.PSObject.Properties.Name -contains "dogego_connect_lag") {
        return [int64]$Info.dogego_connect_lag
    }
    if ($Info.PSObject.Properties.Name -contains "dogego_stored_bodies_ahead_connect") {
        return [int64]$Info.dogego_stored_bodies_ahead_connect
    }
    if ($Info.PSObject.Properties.Name -contains "dogego_contiguous_raw_height" -and
        $Info.PSObject.Properties.Name -contains "blocks") {
        $c = [int64]$Info.dogego_contiguous_raw_height
        $b = [int64]$Info.blocks
        if ($c -gt $b) { return $c - $b }
    }
    return $null
}

function Get-DogeGoRpcConnectBlocksPerMinute {
    param($Info)
    if ($null -eq $Info) { return $null }
    if ($Info.PSObject.Properties.Name -contains "dogego_connect_blocks_per_minute") {
        return [double]$Info.dogego_connect_blocks_per_minute
    }
    if ($Info.PSObject.Properties.Name -contains "dogego_raw_sync") {
        $rs = $Info.dogego_raw_sync
        if ($null -ne $rs -and $rs.PSObject.Properties.Name -contains "connect_blocks_per_minute") {
            return [double]$rs.connect_blocks_per_minute
        }
    }
    return $null
}

# Active connect catch-up tuning from getblockchaininfo (passes×batch when lag>0).
function Get-DogeGoRpcConnectCatchUpTuning {
    param($Info)
    $out = [ordered]@{
        passes      = $null
        batch       = $null
        interval_ms = $null
    }
    if ($null -eq $Info) { return $out }
    if ($Info.PSObject.Properties.Name -contains "dogego_connect_catch_up_passes") {
        $out.passes = [int64]$Info.dogego_connect_catch_up_passes
    }
    if ($Info.PSObject.Properties.Name -contains "dogego_connect_catch_up_batch") {
        $out.batch = [int64]$Info.dogego_connect_catch_up_batch
    }
    if ($Info.PSObject.Properties.Name -contains "dogego_connect_catch_up_interval_ms") {
        $out.interval_ms = [int64]$Info.dogego_connect_catch_up_interval_ms
    }
    return $out
}

function Format-DogeGoConnectCatchUpBoost {
    param($Tuning)
    if ($null -eq $Tuning) { return $null }
    $passes = $Tuning.passes
    $batch = $Tuning.batch
    $ms = $Tuning.interval_ms
    if ($null -eq $passes -or $passes -le 0 -or $null -eq $batch -or $batch -le 0) { return $null }
    $line = ("{0}x{1}" -f $passes, $batch)
    if ($null -ne $ms -and $ms -gt 0) { $line += (" @{0}ms" -f $ms) }
    return $line
}

# Human-readable body IBD ETA from headers tip, contiguous bodies, and download rate.
function Format-DogeGoBodyIBDEta {
    param(
        [int64]$HeaderTip,
        [int64]$Contiguous,
        [double]$BlocksPerMinute
    )
    if ($HeaderTip -le 0 -or $Contiguous -lt 0 -or $BlocksPerMinute -le 0) { return $null }
    $behind = $HeaderTip - $Contiguous
    if ($behind -le 0) { return $null }
    $etaMin = [math]::Ceiling($behind / $BlocksPerMinute)
    if ($etaMin -lt 60) {
        if ($etaMin -le 1) { return "about 1 minute" }
        return ("about {0} minutes" -f $etaMin)
    }
    if ($etaMin -lt 1440) {
        $h = [math]::Ceiling($etaMin / 60.0)
        if ($h -le 1) { return "about 1 hour" }
        return ("about {0} hours" -f $h)
    }
    $d = [math]::Ceiling($etaMin / 1440.0)
    if ($d -le 1) { return "about 1 day" }
    return ("about {0} days" -f $d)
}

function Get-DogeGoBodyIBDEtaMinutes {
    param(
        [int64]$HeaderTip,
        [int64]$Contiguous,
        [double]$BlocksPerMinute
    )
    if ($HeaderTip -le 0 -or $Contiguous -lt 0 -or $BlocksPerMinute -le 0) { return $null }
    $behind = $HeaderTip - $Contiguous
    if ($behind -le 0) { return $null }
    return [int64][math]::Ceiling($behind / $BlocksPerMinute)
}

# Body IBD pump snapshot from getblockchaininfo (download rate, ETA, assist pool).
function Get-DogeGoBodyIBDSnapshot {
    param($Info)
    $out = [ordered]@{}
    if ($null -eq $Info) { return $out }
    $headers = $null
    $cont = $null
    if ($Info.PSObject.Properties.Name -contains "headers") { $headers = [int64]$Info.headers }
    if ($Info.PSObject.Properties.Name -contains "dogego_contiguous_raw_height") {
        $cont = [int64]$Info.dogego_contiguous_raw_height
    }
    if ($Info.dogego_raw_sync) {
        $rs = $Info.dogego_raw_sync
        if ($rs.blocks_per_minute) { $out.download_per_min = [math]::Round([double]$rs.blocks_per_minute, 2) }
        if ($null -ne $rs.in_flight_batches) { $out.in_flight = [int]$rs.in_flight_batches }
        if ($null -ne $rs.assist_active_sessions) { $out.assist_sessions = [int]$rs.assist_active_sessions }
        if ($null -ne $rs.assist_peer_pool) { $out.assist_pool = [int]$rs.assist_peer_pool }
        if ($rs.last_block_stored_at) {
            $out.last_body_store_min = [math]::Round(((Get-Date) - [DateTimeOffset]::FromUnixTimeSeconds([int64]$rs.last_block_stored_at).LocalDateTime).TotalMinutes, 1)
        }
        if ($null -ne $headers -and $null -ne $cont -and $rs.blocks_per_minute -gt 0) {
            $out.body_behind = $headers - $cont
            if ($out.body_behind -gt 0) {
                $out.body_pct = [math]::Round(100.0 * $cont / $headers, 2)
                $out.body_eta_text = Format-DogeGoBodyIBDEta -HeaderTip $headers -Contiguous $cont -BlocksPerMinute ([double]$rs.blocks_per_minute)
                $etaMin = Get-DogeGoBodyIBDEtaMinutes -HeaderTip $headers -Contiguous $cont -BlocksPerMinute ([double]$rs.blocks_per_minute)
                if ($null -ne $etaMin) { $out.body_eta_min = $etaMin }
            }
        }
    }
    if ($Info.PSObject.Properties.Name -contains "dogego_body_ibd_eta_minutes") {
        $out.body_eta_min_rpc = [int64]$Info.dogego_body_ibd_eta_minutes
    }
    if ($Info.PSObject.Properties.Name -contains "dogego_body_ibd_header_paused") {
        $out.header_paused = [bool]$Info.dogego_body_ibd_header_paused
    }
    if ($out.header_paused -eq $true -and $null -ne $headers -and $headers -ge 500000 -and $null -ne $cont) {
        $resumeCont = $headers - 50000
        if ($resumeCont -gt $cont) {
            $out.header_resume_contiguous = $resumeCont
            $out.header_resume_blocks = $resumeCont - $cont
            if ($out.download_per_min -gt 0) {
                $out.header_resume_eta_text = Format-DogeGoBodyIBDEta -HeaderTip $resumeCont -Contiguous $cont -BlocksPerMinute ([double]$out.download_per_min)
            }
        }
    }
    return $out
}

# Latest chainActive from Web UI logs when RPC/web summary is warming or timing out during heavy connect.
function Get-DogeGoLatestConnectHeightFromLogs {
    param(
        [string]$WebUI,
        [int]$Limit = 300
    )
    $base = if ($WebUI) { $WebUI.TrimEnd('/') } else { (Get-DogeGoWebUIUrl).TrimEnd('/') }
    $r = Invoke-RestMethod -Uri ($base + "/api/logs?limit=$Limit") -TimeoutSec 12
    $best = $null
    foreach ($line in @($r.lines)) {
        $msg = [string]$line.msg
        if ($msg -match 'Connected block height (\d+)') {
            $h = [int64]$Matches[1]
            if ($null -eq $best -or $h -gt $best) { $best = $h }
        }
    }
    return $best
}

# Best-effort chainActive + stored heights during IBD (RPC, web, logs).
function Get-DogeGoSyncHeightsSnapshot {
    param(
        [int]$RpcTimeoutSec = 90,
        [int]$RpcWarmupRetries = 2
    )
    $out = [ordered]@{
        blocks = $null
        stored = $null
        lag    = $null
        source = $null
    }
    $info = $null
    try {
        $info = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries $RpcWarmupRetries -WarmupDelaySec 3 -TimeoutSec $RpcTimeoutSec
        $blocks = [int64]$info.blocks
        if ($info.PSObject.Properties.Name -contains "dogego_utxo_chain_active") {
            $utxoH = [int64]$info.dogego_utxo_chain_active
            if ($utxoH -gt $blocks) { $blocks = $utxoH }
        }
        if ($blocks -gt 0) {
            $out.blocks = $blocks
            $out.source = "rpc"
        }
        if ($info.PSObject.Properties.Name -contains "dogego_contiguous_raw_height") {
            $out.stored = [int64]$info.dogego_contiguous_raw_height
        }
        $out.lag = Get-DogeGoRpcConnectLag $info
    } catch {
        $out.source = "rpc_fail"
    }
    if ($null -eq $out.blocks -or $out.blocks -le 0) {
        try {
            $web = Get-DogeGoWebSummary
            if ($null -ne $web.chain_active_height -and [string]$web.chain_active_height -ne "") {
                $wh = [int64]$web.chain_active_height
                if ($wh -gt 0) {
                    $out.blocks = $wh
                    $out.source = if ($out.source -eq "rpc_fail") { "web" } else { "rpc+web" }
                }
            }
            if ($null -eq $out.stored -and $null -ne $web.contiguous_raw_height) {
                $out.stored = [int64]$web.contiguous_raw_height
            }
        } catch { }
    }
    if ($null -eq $out.blocks -or ($out.blocks -le 0 -and $null -ne $out.stored -and $out.stored -gt 1000)) {
        try {
            $logH = Get-DogeGoLatestConnectHeightFromLogs
            if ($null -ne $logH -and $logH -gt 0) {
                $out.blocks = $logH
                $out.source = if ($out.source) { "$($out.source)+logs" } else { "logs" }
            }
        } catch { }
    }
    if ($null -eq $out.lag -and $null -ne $out.blocks -and $null -ne $out.stored -and $out.stored -gt $out.blocks) {
        $out.lag = $out.stored - $out.blocks
    }
    return [pscustomobject]$out
}
