# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Core wallet.dat migration certification (native BDB probe/import + UI API).
#
# Cross-platform: dogego cert wallet-migration
#   go run ./cmd/dogego cert wallet-migration
#   dogego cert wallet-migration -wallet-dat "C:\path\wallet.dat" -passphrase "yourpass"
#
#   .\scripts\wallet_migration_cert.ps1
#
# Optional live probe against a real wallet.dat on disk:
#   $env:DOGEGO_WALLET_DAT = "C:\path\to\wallet.dat"
#   $env:DOGEGO_WALLET_DAT_PASSPHRASE = "yourpass"   # for encrypted wallet.dat
#   .\scripts\wallet_migration_cert.ps1
param(
    [switch]$Json,
    [switch]$SkipOffline,
    [switch]$RequireWalletDat,
    [string]$WalletDatPath = $env:DOGEGO_WALLET_DAT,
    [string]$WalletDatPassphrase = $env:DOGEGO_WALLET_DAT_PASSPHRASE
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\_wallet_dat_env.ps1"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

if (-not $WalletDatPath) {
    $WalletDatPath = Initialize-WalletDatEnv
}

$results = [ordered]@{
    offline_tests = "skipped"
    live_probe = "skipped"
    live_import = "skipped"
}

Write-Host "=== Core wallet.dat migration certification ===" -ForegroundColor Cyan

if (-not $SkipOffline) {
    $results.offline_tests = "pending"
    Write-Host "`n> go run ./cmd/dogego cert wallet-migration -offline-only" -ForegroundColor Yellow
    go run ./cmd/dogego cert wallet-migration -offline-only
    if ($LASTEXITCODE -ne 0) {
        Write-Host "FAILED: wallet-migration offline cert" -ForegroundColor Red
        if ($Json) {
            $results.offline_tests = "failed"
            $results | ConvertTo-Json -Depth 4
        }
        exit $LASTEXITCODE
    }
    $results.offline_tests = "passed"
}

if ($WalletDatPath -and (Test-Path -LiteralPath $WalletDatPath)) {
    Write-Host "`n> Live probe: $WalletDatPath" -ForegroundColor Yellow
    . "$PSScriptRoot\dogego_rpc.ps1"
    try {
        $probe = Invoke-DogeGoJsonRpc -Method dogego_probewalletdat -Params @($WalletDatPath)
        if ($probe.is_bdb -ne $true) {
            Write-Host "Live probe: not recognized as BDB wallet.dat" -ForegroundColor Red
            $results.live_probe = "not_bdb"
            if ($Json) { $results | ConvertTo-Json -Depth 4 }
            exit 1
        }
        $results.live_probe = "passed"
        $poolMeta = ""
        if ($probe.pool_count -gt 0) {
            if ($probe.pool_pubkeys -gt 0) { $poolMeta += " pool_pubkeys=$($probe.pool_pubkeys)" }
            if ($probe.pool_keys_matched -gt 0) { $poolMeta += " pool_keys_matched=$($probe.pool_keys_matched)" }
            if ($probe.pool_keys_unmatched -gt 0) { $poolMeta += " pool_keys_unmatched=$($probe.pool_keys_unmatched)" }
            if ($null -ne $probe.pool_entries -and $probe.pool_entries.Count -gt 0) { $poolMeta += " pool_entries=$($probe.pool_entries.Count)" }
            if ($null -ne $probe.pool_index_min) {
                if ($probe.pool_index_min -eq $probe.pool_index_max) {
                    $poolMeta += " pool_idx=$($probe.pool_index_min)"
                } else {
                    $poolMeta += " pool_idx=$($probe.pool_index_min)-$($probe.pool_index_max)"
                }
            }
            if ($null -ne $probe.pool_indices_replayed) { $poolMeta += " pool_indices_replayed=$($probe.pool_indices_replayed)" }
        }
        Write-Host ("  keys={0} watch={1} encrypted={2} encrypted_keys={3} pool={4}{5} needs_passphrase={6} can_import={7}" -f $probe.key_count, $probe.watch_count, $probe.encrypted, $probe.encrypted_keys, $probe.pool_count, $poolMeta, $probe.needs_passphrase, $probe.can_import) -ForegroundColor DarkGray
        if ($probe.can_import -eq $true -and $probe.encrypted -ne $true) {
            Write-Host "`n> Live import (native BDB): $WalletDatPath" -ForegroundColor Yellow
            $import = Invoke-DogeGoJsonRpc -Method dogego_importwalletdat -Params @($WalletDatPath, '{"native_bdb":true}')
            if ($null -eq $import) {
                Write-Host "Live import returned null" -ForegroundColor Red
                $results.live_import = "failed"
                if ($Json) { $results | ConvertTo-Json -Depth 4 }
                exit 1
            }
            $results.live_import = "passed"
            if ($import.PSObject.Properties.Name -contains "keys_imported") {
                Write-Host ("  keys_imported={0}" -f $import.keys_imported) -ForegroundColor DarkGray
            }
            if ($import.keypool_hint) { Write-Host ("  keypool_hint: {0}" -f $import.keypool_hint) -ForegroundColor DarkGray }
        } elseif ($probe.needs_passphrase -eq $true -and $WalletDatPassphrase) {
            Write-Host "`n> Live import (encrypted native): $WalletDatPath" -ForegroundColor Yellow
            $opts = (@{ passphrase = $WalletDatPassphrase } | ConvertTo-Json -Compress)
            $import = Invoke-DogeGoJsonRpc -Method dogego_importwalletdat -Params @($WalletDatPath, $opts)
            if ($null -eq $import) {
                Write-Host "Live encrypted import returned null" -ForegroundColor Red
                $results.live_import = "failed"
                if ($Json) { $results | ConvertTo-Json -Depth 4 }
                exit 1
            }
            $results.live_import = "passed_encrypted"
            if ($import.PSObject.Properties.Name -contains "keys_imported") {
                Write-Host ("  keys_imported={0}" -f $import.keys_imported) -ForegroundColor DarkGray
            }
            if ($import.keypool_hint) { Write-Host ("  keypool_hint: {0}" -f $import.keypool_hint) -ForegroundColor DarkGray }
        } elseif ($probe.needs_passphrase -eq $true) {
            $results.live_import = "skipped_needs_passphrase"
            Write-Host "Live import skipped (encrypted - set DOGEGO_WALLET_DAT_PASSPHRASE)" -ForegroundColor DarkGray
        } else {
            $results.live_import = "skipped_encrypted_or_blocked"
            Write-Host "Live import skipped (encrypted or can_import=false)" -ForegroundColor DarkGray
        }
    } catch {
        Write-Host "Live probe/import failed: $($_.Exception.Message)" -ForegroundColor Red
        $results.live_probe = "failed"
        if ($Json) { $results | ConvertTo-Json -Depth 4 }
        exit 1
    }
} elseif ($WalletDatPath) {
    Write-Host "`nWallet path not found (skipping live probe): $WalletDatPath" -ForegroundColor DarkGray
    $results.live_probe = "path_missing"
} elseif ($RequireWalletDat -or $env:DOGEGO_WALLET_DAT_REQUIRED -eq "1") {
    Write-Host "`nwallet.dat required but not found (set DOGEGO_WALLET_DAT or run provision_wallet_dat_fixture.ps1)" -ForegroundColor Red
    $results.live_probe = "required_missing"
    if ($Json) { $results | ConvertTo-Json -Depth 4 }
    exit 1
}

if (($RequireWalletDat -or $env:DOGEGO_WALLET_DAT_REQUIRED -eq "1") -and $results.live_import -match "^skipped") {
    Write-Host "`nwallet.dat live import required but skipped ($($results.live_import))" -ForegroundColor Red
    if ($Json) { $results | ConvertTo-Json -Depth 4 }
    exit 1
}

Write-Host "`nWallet migration certification passed." -ForegroundColor Green
if ($Json) { $results | ConvertTo-Json -Depth 4 }
exit 0
