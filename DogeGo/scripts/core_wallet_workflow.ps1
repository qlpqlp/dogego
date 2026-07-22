# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E (partial): Core wallet basics when DogeGo wallet is enabled.
#
#   .\scripts\core_wallet_workflow.ps1
param(
    [switch]$Json,
    [string]$DataDir,
    [string]$Network = "mainnet",
    [string]$WalletDatPath = $env:DOGEGO_WALLET_DAT,
    [string]$WalletDatPassphrase = $env:DOGEGO_WALLET_DAT_PASSPHRASE
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"
. "$PSScriptRoot\_wallet_dat_env.ps1"

function Get-PqCommitmentTagFromTxHex {
    param([string]$Hex)
    if (-not $Hex) { return $null }
    $h = ($Hex -replace '\s', '').ToLower()
    foreach ($pair in @(
            @{ Pattern = '6a24464c4331'; Tag = 'FLC1' }
            @{ Pattern = '6a2444494c32'; Tag = 'DIL2' }
            @{ Pattern = '6a2452434734'; Tag = 'RCG4' }
        )) {
        if ($h -match $pair.Pattern) { return $pair.Tag }
    }
    return $null
}

if (-not $WalletDatPath) {
    $WalletDatPath = Initialize-WalletDatEnv
}

$issues = @()
$warnings = @()
$notes = @()

try {
    $w = Invoke-DogeGoJsonRpc -Method getwalletinfo -WarmupRetries 3 -WarmupDelaySec 1
} catch {
    if ($_.Exception.Message -match "not implemented|-1") {
        Write-Host "Wallet not enabled - skipping core wallet workflow." -ForegroundColor DarkGray
        exit 0
    }
    $issues += "getwalletinfo_failed"
    $w = $null
}

$bal = $null
$addr = $null
$addressBookCount = $null
$addressBookKeypoolCount = $null
$addressBookCorePoolIndicesStored = $null
$keypoolValidateAddressOK = $null
$keypoolGetAddressInfoOK = $null
$signerCmdConfigured = $false
$signerConfigured = $false
$walletScanIndexOK = $null
$walletHistoryFastPath = $null
$walletListtransactionsUtxoWalk = $null
$walletListtransactionsScanPending = $null
if ($w) {
    if ($w.PSObject.Properties.Name -contains "signer_cmd_configured" -and $w.signer_cmd_configured -eq $true) {
        $signerCmdConfigured = $true
    }
    $pqCommitmentsEnabled = $null
    $pqCommitmentsOK = $null
    if ($w.PSObject.Properties.Name -contains "pq_commitments_enabled") {
        $pqCommitmentsEnabled = [bool]$w.pq_commitments_enabled
        $pqCommitmentsOK = ($pqCommitmentsEnabled -eq $true)
        if (-not $pqCommitmentsOK) {
            $notes += "pq_commitments_disabled: enable under Settings -> Wallet or setwalletflag pq_commitments true"
        }
    } else {
        $pqCommitmentsEnabled = $true
        $pqCommitmentsOK = $true
    }
    if ($w.PSObject.Properties.Name -contains "scanning") {
        $warnings += "wallet_rescan_in_progress"
    }
    if ($w.needs_rescan -eq $true) {
        if ($walletHistoryFastPath) {
            $notes += "wallet_index_lags_chain: rescan backfills fee/hex metadata; listtransactions already uses partial scan index"
        } else {
            $notes += "wallet_index_lags_chain: run rescan RPC or POST /api/wallet/rescan to backfill fee/hex metadata; listtransactions may walk all UTXOs until indexed"
        }
    }
    if ($w.PSObject.Properties.Name -contains "dogego_wallet_scan_index_ok") {
        $walletScanIndexOK = [bool]$w.dogego_wallet_scan_index_ok
        if ($walletScanIndexOK) {
            $notes += "wallet_scan_index_ok: listtransactions uses wallet.db history (no per-UTXO receive walk)"
        }
    }
    if ($w.PSObject.Properties.Name -contains "dogego_wallet_history_fast_path" -and $w.dogego_wallet_history_fast_path -eq $true) {
        $walletHistoryFastPath = $true
        if (-not $walletScanIndexOK) {
            $notes += "wallet_history_fast_path: listtransactions skips UTXO receive walk (partial scan index)"
        }
    }
    if ($w.PSObject.Properties.Name -contains "dogego_wallet_listtransactions_utxo_walk" -and $w.dogego_wallet_listtransactions_utxo_walk -eq $true) {
        $walletListtransactionsUtxoWalk = $true
        $spendable = 0
        if ($w.PSObject.Properties.Name -contains "spendable_utxo_count") {
            $spendable = [int]$w.spendable_utxo_count
        }
        if ($spendable -gt 64) {
            $notes += "wallet_listtransactions_utxo_walk: listtransactions walks UTXO cache for receive rows; rescan recommended for solo miners with many coinbases"
        } else {
            $notes += "wallet_listtransactions_utxo_walk: listtransactions walks UTXO cache for receive rows until wallet.db scan index exists"
        }
    }
    if ($w.PSObject.Properties.Name -contains "dogego_wallet_listtransactions_scan_pending" -and $w.dogego_wallet_listtransactions_scan_pending -eq $true) {
        $walletListtransactionsScanPending = $true
    }
    if ($w.PSObject.Properties.Name -contains "scanning" -and $walletListtransactionsUtxoWalk) {
        $notes += "wallet_scan_building_index: listtransactions may walk UTXOs until rescan populates wallet.db receive rows"
    }
    try { $bal = Invoke-DogeGoJsonRpc -Method getbalance } catch { $warnings += "getbalance_failed" }
    try { $addr = Invoke-DogeGoJsonRpc -Method getnewaddress -Params @("receive") } catch {
        try { $addr = Invoke-DogeGoJsonRpc -Method getaccountaddress -Params @("") } catch { $warnings += "getaddress_failed" }
    }
    $keypoolTopupOK = $null
    $keypoolSizeAfter = $null
    if ($addr) {
        try {
            $wi2 = Invoke-DogeGoJsonRpc -Method getwalletinfo
            if ($wi2 -and $wi2.format -eq "hd") {
                $keypoolSizeAfter = [int]$wi2.keypoolsize
                if ($wi2.keypoolsize -ge 50) {
                    $keypoolTopupOK = $true
                } else {
                    $warnings += "keypool_below_threshold"
                }
            }
        } catch {
            $warnings += "getwalletinfo_keypool_check_failed"
        }
    }
    if ($addr) {
        try {
            $v = Invoke-DogeGoJsonRpc -Method validateaddress -Params @($addr)
            if ($v -and $v.isvalid -ne $true) { $issues += "validateaddress_invalid" }
        } catch {
            $warnings += "validateaddress_failed"
        }
        try {
            Invoke-DogeGoJsonRpc -Method setlabel -Params @($addr, "dogego-probe") | Out-Null
            $byLabel = Invoke-DogeGoJsonRpc -Method getaddressesbylabel -Params @("dogego-probe")
            $found = $false
            if ($byLabel -and $byLabel.PSObject.Properties.Name -contains $addr) {
                $found = $true
            } elseif ($byLabel -is [System.Collections.IEnumerable]) {
                foreach ($row in $byLabel) {
                    if ("$row" -eq "$addr") { $found = $true; break }
                }
            }
            if (-not $found) { $issues += "setlabel_roundtrip_mismatch" } else {
                try {
                    $labels = Invoke-DogeGoJsonRpc -Method listlabels
                    $labelFound = $false
                    if ($labels -is [System.Collections.IEnumerable]) {
                        foreach ($lbl in $labels) {
                            if ("$lbl" -eq "dogego-probe") { $labelFound = $true; break }
                        }
                    }
                    if (-not $labelFound) { $issues += "listlabels_roundtrip_mismatch" }
                } catch {
                    $warnings += "listlabels_failed"
                }
            }
            Invoke-DogeGoJsonRpc -Method setlabel -Params @($addr, "") | Out-Null
        } catch {
            $warnings += "setlabel_roundtrip_failed"
        }
    }
    try {
        $rows = Invoke-DogeGoJsonRpc -Method dogego_listwalletaddresses
        if ($null -eq $rows) { $warnings += "dogego_listwalletaddresses_empty" }
        else {
            $addressRows = @($rows)
            $addressBookCount = $addressRows.Count
            $keypoolCount = 0
            $corePoolIndexCount = 0
            foreach ($row in $addressRows) {
                if ($row.iskeypool -eq $true) { $keypoolCount++ }
                if ($row.PSObject.Properties.Name -contains "hd_keypool_core_index" -and $null -ne $row.hd_keypool_core_index) {
                    $corePoolIndexCount++
                }
            }
            $addressBookKeypoolCount = $keypoolCount
            $addressBookCorePoolIndicesStored = $corePoolIndexCount
            $keypoolAddr = $null
            $keypoolCoreIdx = $null
            foreach ($row in $addressRows) {
                if ($row.iskeypool -eq $true -and $row.address) {
                    $keypoolAddr = "$($row.address)"
                    if ($row.PSObject.Properties.Name -contains "hd_keypool_core_index" -and $null -ne $row.hd_keypool_core_index) {
                        $keypoolCoreIdx = $row.hd_keypool_core_index
                    }
                    break
                }
            }
            if ($keypoolAddr) {
                try {
                    $va = Invoke-DogeGoJsonRpc -Method validateaddress -Params @($keypoolAddr)
                    if ($va -and $va.isvalid -eq $true -and $va.iskeypool -eq $true) {
                        $keypoolValidateAddressOK = $true
                        if ($null -ne $keypoolCoreIdx -and $va.hd_keypool_core_index -ne $keypoolCoreIdx) {
                            $issues += "keypool_validateaddress_core_index_mismatch"
                            $keypoolValidateAddressOK = $false
                        }
                    } else {
                        $issues += "keypool_validateaddress_mismatch"
                    }
                    $ai = Invoke-DogeGoJsonRpc -Method getaddressinfo -Params @($keypoolAddr)
                    if ($ai -and $ai.iskeypool -eq $true) {
                        $keypoolGetAddressInfoOK = $true
                        if ($null -ne $keypoolCoreIdx -and $ai.hd_keypool_core_index -ne $keypoolCoreIdx) {
                            $issues += "keypool_getaddressinfo_core_index_mismatch"
                            $keypoolGetAddressInfoOK = $false
                        }
                    } else {
                        $issues += "keypool_getaddressinfo_mismatch"
                    }
                } catch {
                    $warnings += "keypool_addressinfo_failed"
                }
            }
            if ($null -ne $addressBookCorePoolIndicesStored -and $w -and $w.PSObject.Properties.Name -contains "pool_core_indices_stored" -and $null -ne $w.pool_core_indices_stored) {
                if ([int]$w.pool_core_indices_stored -ne [int]$addressBookCorePoolIndicesStored) {
                    $issues += "pool_core_indices_count_mismatch"
                }
            }
        }
    } catch {
        $warnings += "dogego_listwalletaddresses_failed"
    }
    try {
        $signers = Invoke-DogeGoJsonRpc -Method enumeratesigners
        if ($null -eq $signers) {
            $warnings += "enumeratesigners_empty"
            if ($signerCmdConfigured) { $notes += "signer_cmd configured but enumeratesigners returned 0 devices" }
        } elseif ($signers -is [System.Collections.IEnumerable]) {
            $n = @($signers).Count
            if ($n -gt 0) {
                $signerConfigured = $true
                Write-Host ("enumeratesigners: " + $n + " device(s)") -ForegroundColor DarkGray
            } elseif ($signerCmdConfigured) {
                $notes += "signer_cmd configured but enumeratesigners returned 0 devices"
            }
        }
    } catch {
        $warnings += "enumeratesigners_failed"
        if ($signerCmdConfigured) { $notes += "signer_cmd configured but enumeratesigners failed (check HWI path and device)" }
    }
}

$walletDatProbe = $null
if ($WalletDatPath -and (Test-Path -LiteralPath $WalletDatPath)) {
    try {
        $walletDatProbe = Invoke-DogeGoJsonRpc -Method dogego_probewalletdat -Params @($WalletDatPath)
        if ($walletDatProbe.is_bdb -ne $true) {
            $warnings += "walletdat_probe_not_bdb"
        } elseif ($walletDatProbe.needs_passphrase -eq $true -and $walletDatProbe.can_import -ne $true) {
            $warnings += "walletdat_probe_encrypted_no_master_key"
        } elseif ($walletDatProbe.can_import -ne $true -and $walletDatProbe.encrypted -ne $true) {
            $warnings += "walletdat_probe_cannot_import"
        } elseif ($walletDatProbe.pool_keys_unmatched -gt 0) {
            $warnings += "walletdat_probe_pool_keys_unmatched"
        }
    } catch {
        $warnings += "walletdat_probe_failed"
    }
} elseif ($WalletDatPath) {
    $warnings += "walletdat_probe_path_missing"
}

$psbtRoundTripOK = $null
$psbtCreateFundedOK = $null
$psbtBIP32DerivOK = $null
if ($addr) {
    try {
        $psbtOut = @{ $addr = 0.001 }
        $created = Invoke-DogeGoJsonRpc -Method walletcreatefundedpsbt -Params @(@(), $psbtOut)
        if ($created -and $created.psbt) {
            $psbtCreateFundedOK = $true
            try {
                $decoded = Invoke-DogeGoJsonRpc -Method decodepsbt -Params @($created.psbt)
                $hasDeriv = $false
                foreach ($section in @("inputs", "outputs")) {
                    if (-not $decoded.$section) { continue }
                    foreach ($row in $decoded.$section) {
                        if ($row.bip32_derivs -and $row.bip32_derivs.Count -gt 0) {
                            $hasDeriv = $true
                            break
                        }
                    }
                    if ($hasDeriv) { break }
                }
                if ($hasDeriv) { $psbtBIP32DerivOK = $true } else { $warnings += "psbt_bip32_deriv_missing" }
            } catch {
                $warnings += "decodepsbt_failed"
            }
            $processed = Invoke-DogeGoJsonRpc -Method walletprocesspsbt -Params @($created.psbt, $true, "ALL", $true, $true)
            if ($processed -and $processed.complete -eq $true) {
                $psbtRoundTripOK = $true
            } else {
                $notes += "psbt_process_incomplete: check signer_cmd or immature inputs"
            }
        }
    } catch {
        $msg = $_.Exception.Message
        if ($msg -match "insufficient|Insufficient") {
            $notes += "psbt_roundtrip_skipped: insufficient mature balance for 0.001 DOGE probe"
        } elseif ($msg -match "unlock|passphrase") {
            $notes += "psbt_roundtrip_skipped: wallet locked (unlock for PSBT probe)"
        } else {
            $warnings += "psbt_roundtrip_failed"
        }
    }
}

$hardwarePsbtHint = $null
if ($signerCmdConfigured -and $psbtCreateFundedOK -eq $true -and $psbtRoundTripOK -ne $true) {
    if ($signerConfigured) {
        $hardwarePsbtHint = "funded PSBT ok; round-trip uses local wallet keys (HWI path not exercised by probe)"
    } else {
        $hardwarePsbtHint = "connect HWI device; funded PSBT has BIP32 deriv paths for external signing"
    }
}

$walletHistoryDeferReason = $null
$walletHistoryDeferred = $false
if ($w) {
    $ibdActive = $false
    $connectLag = 0
    try {
        $chain = Invoke-DogeGoJsonRpc -Method getblockchaininfo
        if ($chain.initialblockdownload -eq $true) { $ibdActive = $true }
        if ($chain.PSObject.Properties.Name -contains "dogego_connect_lag") {
            $connectLag = [int64]$chain.dogego_connect_lag
        }
    } catch {
        # optional chain snapshot for defer parity
    }
    if ($ibdActive) {
        $walletHistoryDeferReason = "ibd_active"
    } elseif ($connectLag -gt 64) {
        $walletHistoryDeferReason = "connect_lag"
    } elseif ($w.PSObject.Properties.Name -contains "scanning") {
        $utxoWalkDefer = ($walletListtransactionsUtxoWalk -eq $true)
        $scanPendingDefer = ($walletListtransactionsScanPending -eq $true)
        if ($utxoWalkDefer -or $scanPendingDefer) {
            $spendableDefer = 0
            if ($w.PSObject.Properties.Name -contains "spendable_utxo_count") {
                $spendableDefer = [int]$w.spendable_utxo_count
            }
            if ($spendableDefer -gt 64) {
                $walletHistoryDeferReason = "scan_building"
            }
        }
    }
    if ($walletHistoryDeferReason) {
        $walletHistoryDeferred = $true
        $notes += "wallet_history_deferred_$($walletHistoryDeferReason): History tab and GET /api/wallet/txs defer listtransactions (same as dashboard)"
    }
}

$walletTxHexOK = $null
$walletTxFeeOK = $null
$walletListTransactionsMs = $null
$walletListTransactionsOK = $null
$walletPqSendOK = $null
$walletPqTag = $null
if ($addr -and -not $walletHistoryDeferred) {
    try {
        $sw = [System.Diagnostics.Stopwatch]::StartNew()
        $txRows = Invoke-DogeGoJsonRpc -Method listtransactions -Params @("*", 40)
        $sw.Stop()
        $walletListTransactionsMs = [int]$sw.ElapsedMilliseconds
        $walletListTransactionsOK = ($walletListTransactionsMs -lt 3000)
        if (-not $walletListTransactionsOK) {
            $notes += "listtransactions_slow: ${walletListTransactionsMs}ms (threshold 3000ms; History tab uses same RPC bridge)"
        }
        $probeTxid = $null
        if ($txRows -is [System.Collections.IEnumerable]) {
            foreach ($row in @($txRows)) {
                if ($row.category -ne "send" -or $row.confirmations -lt 1 -or -not $row.txid) { continue }
                if (-not $probeTxid) { $probeTxid = $row.txid }
                if ($pqCommitmentsOK -eq $true -and $walletPqSendOK -ne $true) {
                    try {
                        $gtPq = Invoke-DogeGoJsonRpc -Method gettransaction -Params @($row.txid)
                        if ($gtPq -and $gtPq.hex) {
                            $tag = Get-PqCommitmentTagFromTxHex -Hex $gtPq.hex
                            if ($tag) {
                                $walletPqSendOK = $true
                                $walletPqTag = $tag
                            }
                        }
                    } catch {
                        # keep scanning other send rows
                    }
                }
            }
        }
        if ($pqCommitmentsOK -eq $true -and $walletPqSendOK -ne $true) {
            if ($probeTxid) {
                $notes += "wallet_pq_send_pending: pq_commitments on but no PQ-tagged send in history yet"
            } else {
                $notes += "wallet_pq_send_skipped: no confirmed send rows"
            }
        }
        if ($probeTxid) {
            $gt = Invoke-DogeGoJsonRpc -Method gettransaction -Params @($probeTxid)
            if ($gt -and $gt.hex) { $walletTxHexOK = $true }
            if ($gt -and $gt.fee -lt 0) { $walletTxFeeOK = $true }
            if (-not $walletTxHexOK) { $notes += "wallet_tx_hex_missing: run POST /api/wallet/rescan or enable tx_index_embed_tx" }
        } else {
            $notes += "wallet_tx_metadata_skipped: no confirmed send rows"
        }
    } catch {
        $warnings += "wallet_tx_metadata_failed"
    }
} elseif ($addr -and $walletHistoryDeferred) {
    $notes += "listtransactions_skipped: wallet_history_deferred"
}

$ok = ($issues.Count -eq 0)
if ($Json) {
    [ordered]@{
        ok       = $ok
        wallet   = $w
        balance  = $bal
        address  = $addr
        address_book_count = $addressBookCount
        address_book_keypool_count = $addressBookKeypoolCount
        address_book_core_pool_indices_stored = $addressBookCorePoolIndicesStored
        keypool_validateaddress_ok = $keypoolValidateAddressOK
        keypool_getaddressinfo_ok = $keypoolGetAddressInfoOK
        signer_cmd_configured = $signerCmdConfigured
        keypool_topup_ok = $keypoolTopupOK
        keypoolsize_after_getnewaddress = $keypoolSizeAfter
        psbt_create_funded_ok = $psbtCreateFundedOK
        psbt_bip32_deriv_ok = $psbtBIP32DerivOK
        psbt_roundtrip_ok = $psbtRoundTripOK
        hardware_psbt_hint = $hardwarePsbtHint
        wallet_tx_hex_ok = $walletTxHexOK
        wallet_tx_fee_ok = $walletTxFeeOK
        wallet_listtransactions_ms = $walletListTransactionsMs
        wallet_listtransactions_ok = $walletListTransactionsOK
        wallet_scan_index_ok = $walletScanIndexOK
        wallet_history_fast_path = $walletHistoryFastPath
        wallet_listtransactions_utxo_walk = $walletListtransactionsUtxoWalk
        wallet_listtransactions_scan_pending = $walletListtransactionsScanPending
        wallet_history_deferred = $walletHistoryDeferred
        wallet_history_defer_reason = $walletHistoryDeferReason
        wallet_pq_send_ok = $walletPqSendOK
        wallet_pq_tag = $walletPqTag
        pq_commitments_enabled = $pqCommitmentsEnabled
        pq_commitments_ok = $pqCommitmentsOK
        wallet_dat_probe = $walletDatProbe
        pool_replay_scan_cap = $(if ($walletDatProbe -and $walletDatProbe.pool_count -gt 0) { 2000 } else { $null })
        notes    = @($notes)
        issues   = @($issues)
        warnings = @($warnings)
    } | ConvertTo-Json -Depth 5
} else {
    Write-Host "=== Core wallet workflow ===" -ForegroundColor Cyan
    if ($w) { Write-Host ("walletname={0} txcount={1}" -f $w.walletname, $w.txcount) }
    if ($null -ne $bal) { Write-Host ("balance={0}" -f $bal) }
    if ($null -ne $addressBookCount) {
        Write-Host ("address_book={0} keypool={1} core_pool_indices={2}" -f $addressBookCount, $addressBookKeypoolCount, $addressBookCorePoolIndicesStored) -ForegroundColor DarkGray
    }
    if ($psbtRoundTripOK -eq $true) {
        Write-Host "PSBT round-trip: walletcreatefundedpsbt + walletprocesspsbt complete" -ForegroundColor DarkGray
    } elseif ($psbtCreateFundedOK -eq $true) {
        Write-Host "PSBT: funded PSBT created (process incomplete)" -ForegroundColor DarkGray
        if ($psbtBIP32DerivOK -eq $true) {
            Write-Host "PSBT BIP32 deriv paths present" -ForegroundColor DarkGray
        }
    }
    if ($walletListTransactionsOK -eq $true) {
        Write-Host ("listtransactions (40 rows): {0} ms" -f $walletListTransactionsMs) -ForegroundColor DarkGray
    }
    if ($walletPqSendOK -eq $true) {
        Write-Host ("PQ send history: {0}" -f $(if ($walletPqTag) { $walletPqTag } else { "sent_pq in tx hex" })) -ForegroundColor DarkGray
    }
    if ($walletDatProbe) {
        $poolMeta = ""
        if ($walletDatProbe.pool_count -gt 0) {
            if ($walletDatProbe.pool_pubkeys -gt 0) { $poolMeta += " pool_pubkeys=$($walletDatProbe.pool_pubkeys)" }
            if ($walletDatProbe.pool_keys_matched -gt 0) { $poolMeta += " pool_keys_matched=$($walletDatProbe.pool_keys_matched)" }
            if ($walletDatProbe.pool_keys_unmatched -gt 0) { $poolMeta += " pool_keys_unmatched=$($walletDatProbe.pool_keys_unmatched)" }
            if ($null -ne $walletDatProbe.pool_entries -and $walletDatProbe.pool_entries.Count -gt 0) { $poolMeta += " pool_entries=$($walletDatProbe.pool_entries.Count)" }
            if ($null -ne $walletDatProbe.pool_index_min) {
                if ($walletDatProbe.pool_index_min -eq $walletDatProbe.pool_index_max) {
                    $poolMeta += " pool_idx=$($walletDatProbe.pool_index_min)"
                } else {
                    $poolMeta += " pool_idx=$($walletDatProbe.pool_index_min)-$($walletDatProbe.pool_index_max)"
                }
            }
            if ($null -ne $walletDatProbe.pool_indices_replayed) { $poolMeta += " pool_indices_replayed=$($walletDatProbe.pool_indices_replayed)" }
            if ($walletDatProbe.pool_count -gt 0) { $poolMeta += " pool_replay_scan_cap=2000" }
        }
        Write-Host ("wallet.dat probe: keys={0} watch={1} encrypted={2} encrypted_keys={3} pool={4}{5} needs_passphrase={6} can_import={7}" -f $walletDatProbe.key_count, $walletDatProbe.watch_count, $walletDatProbe.encrypted, $walletDatProbe.encrypted_keys, $walletDatProbe.pool_count, $poolMeta, $walletDatProbe.needs_passphrase, $walletDatProbe.can_import) -ForegroundColor DarkGray
        if ($walletDatProbe.needs_passphrase -eq $true -and -not $WalletDatPassphrase) {
            Write-Host "  hint: set DOGEGO_WALLET_DAT_PASSPHRASE for native encrypted import" -ForegroundColor DarkGray
        }
        if ($walletDatProbe.pool_count -gt 0) {
            if ($walletDatProbe.hint) {
                Write-Host ("  keypool_hint: {0}" -f $walletDatProbe.hint) -ForegroundColor DarkGray
            }
            if ($walletDatProbe.pool_unmatched_hint) {
                Write-Host ("  pool_unmatched_hint: {0}" -f $walletDatProbe.pool_unmatched_hint) -ForegroundColor DarkGray
            }
        }
    }
    foreach ($i in $issues) { Write-Host ("FAIL: " + $i) -ForegroundColor Red }
    foreach ($x in $warnings) { Write-Host ("WARN: " + $x) -ForegroundColor Yellow }
    foreach ($n in $notes) { Write-Host ("NOTE: " + $n) -ForegroundColor DarkGray }
    if ($ok) { Write-Host "`nCore wallet workflow passed." -ForegroundColor Green }
    else { Write-Host "`nCore wallet workflow failed." -ForegroundColor Red }
}

if (-not $ok) { exit 1 }
exit 0
