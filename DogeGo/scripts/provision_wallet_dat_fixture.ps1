# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Provision a Core wallet.dat path for dogego-live weekly migration probes.
#
#   .\scripts\provision_wallet_dat_fixture.ps1 -CoreDataDir "C:\path\to\core\testnet"
#   .\scripts\provision_wallet_dat_fixture.ps1 -WalletDatPath "C:\path\wallet.dat" -Passphrase "secret"
#   .\scripts\provision_wallet_dat_fixture.ps1 -SetUserEnv
#
# Sets (or prints) DOGEGO_WALLET_DAT / DOGEGO_WALLET_DAT_PASSPHRASE and runs:
#   dogego cert wallet-migration -skip-offline -wallet-dat PATH [-passphrase]
param(
    [string]$CoreDataDir,
    [string]$WalletDatPath,
    [string]$Passphrase = $env:DOGEGO_WALLET_DAT_PASSPHRASE,
    [string]$Network = "reboottestnet",
    [switch]$SetUserEnv,
    [switch]$Json
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\_wallet_dat_env.ps1"

$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

$path = Resolve-WalletDatPath -Explicit $WalletDatPath -CoreDir $CoreDataDir
if (-not $path) {
    Write-Error "wallet.dat not found. Pass -WalletDatPath or -CoreDataDir (directory containing wallet.dat)."
}

Write-Host "=== Provision Core wallet.dat fixture ===" -ForegroundColor Cyan
Write-Host "path: $path" -ForegroundColor DarkGray

$env:DOGEGO_WALLET_DAT = $path
if ($Passphrase) { $env:DOGEGO_WALLET_DAT_PASSPHRASE = $Passphrase }

if ($SetUserEnv) {
    [Environment]::SetEnvironmentVariable("DOGEGO_WALLET_DAT", $path, "User")
    if ($Passphrase) {
        [Environment]::SetEnvironmentVariable("DOGEGO_WALLET_DAT_PASSPHRASE", $Passphrase, "User")
    }
    Write-Host "Set User env DOGEGO_WALLET_DAT=$path" -ForegroundColor Green
}

$walletArgs = @("cert", "wallet-migration", "-skip-offline", "-wallet-dat", $path, "-network", $Network, "-json")
if ($Passphrase) { $walletArgs += @("-passphrase", $Passphrase) }

$probeOut = & go run ./cmd/dogego @walletArgs 2>&1
$probeOk = ($LASTEXITCODE -eq 0)

$report = [ordered]@{
    ok           = $probeOk
    path         = $path
    network      = $Network
    has_passphrase = [bool]$Passphrase
    probe        = if ($probeOk) { ($probeOut | Out-String).Trim() } else { ($probeOut | Out-String).Trim() }
    env_hint     = "DOGEGO_WALLET_DAT=$path"
}

if ($Json) {
    $report | ConvertTo-Json -Depth 6
} else {
    if ($probeOk) {
        Write-Host "wallet.dat probe passed." -ForegroundColor Green
        Write-Host "Optional: set DOGEGO_WALLET_DAT_REQUIRED=1 and run dogego cert weekly -require-wallet-dat" -ForegroundColor DarkGray
    } else {
        Write-Host "wallet.dat probe failed:" -ForegroundColor Red
        Write-Host ($probeOut | Out-String)
    }
}

if (-not $probeOk) { exit 1 }
exit 0
