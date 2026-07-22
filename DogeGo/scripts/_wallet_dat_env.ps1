# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Shared Core wallet.dat path resolution for dogego-live scripts.

function Resolve-WalletDatPath {
    param(
        [string]$Explicit,
        [string]$CoreDir
    )
    if ($Explicit -and (Test-Path -LiteralPath $Explicit)) {
        return (Resolve-Path -LiteralPath $Explicit).Path
    }
    $candidates = @()
    if ($CoreDir) {
        $candidates += (Join-Path $CoreDir "wallet.dat")
    }
    if ($env:DOGEGO_CORE_DATADIR) {
        $candidates += (Join-Path $env:DOGEGO_CORE_DATADIR "wallet.dat")
    }
    if ($env:APPDATA) {
        $candidates += (Join-Path $env:APPDATA "Dogecoin\testnet3\wallet.dat")
        $candidates += (Join-Path $env:APPDATA "Dogecoin\wallet.dat")
    }
    if ($env:HOME) {
        $candidates += (Join-Path $env:HOME ".dogecoin\testnet3\wallet.dat")
        $candidates += (Join-Path $env:HOME ".dogecoin\wallet.dat")
    }
    foreach ($p in $candidates) {
        if ($p -and (Test-Path -LiteralPath $p)) { return (Resolve-Path -LiteralPath $p).Path }
    }
    return $null
}

function Initialize-WalletDatEnv {
    if ($env:DOGEGO_WALLET_DAT -and (Test-Path -LiteralPath $env:DOGEGO_WALLET_DAT)) {
        return $env:DOGEGO_WALLET_DAT
    }
    $path = Resolve-WalletDatPath -Explicit "" -CoreDir ""
    if ($path) {
        $env:DOGEGO_WALLET_DAT = $path
    }
    return $path
}
