# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Export mainnet block hex into consensus/testdata/mainnet_field_blocks.json.
# Prefers local rawblocks/ via UPDATE_CORE_TESTDATA when bodies exist; else DogeGo/Core RPC.
#
#   .\scripts\export_mainnet_field_blocks.ps1
#   .\scripts\export_mainnet_field_blocks.ps1 -RpcOnly -Heights @(1,2,3)
param(
    [switch]$RpcOnly,
    [switch]$CanonicalOnly,
    [int[]]$Heights = @(1, 2, 3, 100, 200, 272, 10006, 15504),
    [string]$DataDir = "",
    [string]$OutFile = ""
)
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

if (-not $OutFile) {
    $OutFile = Join-Path $DogeGo "consensus\testdata\mainnet_field_blocks.json"
}

if (-not $DataDir) {
    $candidates = @(
        (Join-Path $DogeGo "dogedata\mainnet"),
        (Join-Path (Split-Path -Parent $DogeGo) "dogedata\mainnet")
    )
    foreach ($c in $candidates) {
        if (Test-Path (Join-Path $c "headers")) {
            $DataDir = $c
            break
        }
    }
}
if ($DataDir) {
    $env:DOGEGO_FIELD_DATADIR = $DataDir
}

function Write-Utf8NoBom([string]$Path, [string]$Text) {
    $enc = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($Path, $Text, $enc)
}

function Export-FromDisk {
    if (-not $DataDir) { return $false }
    $syncPath = Join-Path $DataDir "rawblocks_sync.json"
    if (-not (Test-Path $syncPath)) { return $false }
    Write-Host "Exporting mainnet_field_blocks.json from disk ($DataDir)..." -ForegroundColor Cyan
    $env:UPDATE_CORE_TESTDATA = "1"
    go test ./consensus -run TestUpdateCoreTestdata -count=1
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    return $true
}

if ($CanonicalOnly) {
    Remove-Item Env:DOGEGO_FIELD_DATADIR -ErrorAction SilentlyContinue
    Write-Host "Exporting mainnet_field_blocks.json from committed canonical specs..." -ForegroundColor Cyan
    $env:UPDATE_CORE_TESTDATA = "1"
    go test ./consensus -run TestUpdateCoreTestdata -count=1
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    Write-Host "Done (canonical). Re-run: .\scripts\field_evidence_cert.ps1" -ForegroundColor Green
    exit 0
}

if (-not $RpcOnly) {
    if (Export-FromDisk) {
        Write-Host "Done (disk). Re-run: go test ./consensus -run TestCoreMainnetField -count=1" -ForegroundColor Green
        exit 0
    }
    Write-Host "No local rawblocks_sync.json - falling back to RPC export" -ForegroundColor Yellow
}

function Invoke-CoreCliBlockHex([int]$Height) {
    $coreCli = $env:DOGEGO_CORE_CLI
    if (-not $coreCli) {
        $coreCli = (Get-Command dogecoin-cli -ErrorAction SilentlyContinue).Source
    }
    if (-not $coreCli) {
        $default = "C:\Program Files\Dogecoin\daemon\dogecoin-cli.exe"
        if (Test-Path $default) { $coreCli = $default }
    }
    if (-not $coreCli) {
        throw "dogecoin-cli not found (set DOGEGO_CORE_CLI)"
    }
    $hash = (& $coreCli getblockhash $Height 2>&1 | Out-String).Trim().Trim('"')
    if ($LASTEXITCODE -ne 0) { throw "getblockhash $Height failed: $hash" }
    $hex = (& $coreCli getblock $hash 0 2>&1 | Out-String).Trim().Trim('"')
    if ($LASTEXITCODE -ne 0) { throw "getblock $Height failed: $hex" }
    return $hex.ToUpper()
}

function Invoke-DogeGoBlockHex([int]$Height) {
    . "$PSScriptRoot\dogego_rpc.ps1"
    $hash = Invoke-DogeGoJsonRpc -Method getblockhash -Params @($Height) -WarmupRetries 2 -WarmupDelaySec 2
    if (-not $hash) { throw "empty hash for height $Height" }
    $hex = Invoke-DogeGoJsonRpc -Method getblock -Params @($hash, 0) -WarmupRetries 1
    if (-not $hex -or $hex.Length -lt 160) { throw "short block hex for height $Height" }
    return $hex.ToUpper()
}

$useCore = $false
try {
    . "$PSScriptRoot\dogego_rpc.ps1"
    $null = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 1
} catch {
    $msg = $_.Exception.Message
    Write-Host "DogeGo RPC unavailable ($msg); fallback dogecoin-cli" -ForegroundColor Yellow
    $useCore = $true
}

$entries = @()
foreach ($h in $Heights) {
    $via = if ($useCore) { "Core" } else { "DogeGo" }
    Write-Host "export height=$h via $via" -ForegroundColor DarkGray
    $hex = if ($useCore) { Invoke-CoreCliBlockHex $h } else { Invoke-DogeGoBlockHex $h }
    $entries += [ordered]@{
        height = $h
        hex    = $hex
    }
    $byteCount = [int]($hex.Length / 2)
    Write-Host "  height $h bytes=$byteCount" -ForegroundColor Green
}

$json = $entries | ConvertTo-Json -Depth 3
Write-Utf8NoBom -Path $OutFile -Text $json
Write-Host "Wrote $($entries.Count) blocks to $OutFile" -ForegroundColor Cyan
exit 0
