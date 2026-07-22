// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"dogego/wallet"
)

// CoreWalletProbeResult mirrors scripts/core_wallet_workflow.ps1 for the Features tab.
type CoreWalletProbeResult struct {
	CheckedAt                        string   `json:"checked_at"`
	Skipped                          bool     `json:"skipped,omitempty"`
	Reason                           string   `json:"reason,omitempty"`
	OK                               bool     `json:"ok"`
	Wallet                           any      `json:"wallet,omitempty"`
	Balance                          any      `json:"balance,omitempty"`
	SpendableUtxoCount               *int     `json:"spendable_utxo_count,omitempty"`
	Address                          string   `json:"address,omitempty"`
	AddressBookCount                 *int     `json:"address_book_count,omitempty"`
	AddressBookKeypoolCount          *int     `json:"address_book_keypool_count,omitempty"`
	AddressBookNodeTipCount          *int     `json:"address_book_node_tip_count,omitempty"`
	AddressBookCorePoolIndicesStored *int     `json:"address_book_core_pool_indices_stored,omitempty"`
	KeypoolValidateAddressOK         bool     `json:"keypool_validateaddress_ok,omitempty"`
	KeypoolGetAddressInfoOK          bool     `json:"keypool_getaddressinfo_ok,omitempty"`
	NodeTipValidateAddressOK         bool     `json:"nodetip_validateaddress_ok,omitempty"`
	NodeTipGetAddressInfoOK          bool     `json:"nodetip_getaddressinfo_ok,omitempty"`
	LabelRoundTripOK                 bool     `json:"label_roundtrip_ok,omitempty"`
	LabelListOK                      bool     `json:"label_list_ok,omitempty"`
	SignerCount                      *int     `json:"signer_count,omitempty"`
	SignerCmdConfigured              bool     `json:"signer_cmd_configured,omitempty"`
	SignerConfigured                 bool     `json:"signer_configured,omitempty"`
	EnumerateSignersOK               bool     `json:"enumeratesigners_ok,omitempty"`
	PsbtCreateFundedOK               bool     `json:"psbt_create_funded_ok,omitempty"`
	PsbtBIP32DerivOK                 bool     `json:"psbt_bip32_deriv_ok,omitempty"`
	PsbtProcessComplete              bool     `json:"psbt_process_complete,omitempty"`
	PsbtRoundTripOK                  bool     `json:"psbt_roundtrip_ok,omitempty"`
	HardwarePsbtHint                 string   `json:"hardware_psbt_hint,omitempty"`
	KeypoolTopupOK                   bool     `json:"keypool_topup_ok,omitempty"`
	KeypoolSizeAfter                 *int     `json:"keypoolsize_after_getnewaddress,omitempty"`
	WalletTxHexOK                    bool     `json:"wallet_tx_hex_ok,omitempty"`
	WalletTxFeeOK                    bool     `json:"wallet_tx_fee_ok,omitempty"`
	WalletListTransactionsMs         int      `json:"wallet_listtransactions_ms,omitempty"`
	WalletListTransactionsOK         bool     `json:"wallet_listtransactions_ok,omitempty"`
	PqCommitmentsEnabled             *bool    `json:"pq_commitments_enabled,omitempty"`
	PqCommitmentsOK                  bool     `json:"pq_commitments_ok,omitempty"`
	WalletPqSendOK                   bool     `json:"wallet_pq_send_ok,omitempty"`
	WalletPqTag                      string   `json:"wallet_pq_tag,omitempty"`
	WalletDatPath                    string   `json:"wallet_dat_path,omitempty"`
	WalletDatProbe                   any      `json:"wallet_dat_probe,omitempty"`
	PoolKeysUnmatched                *int     `json:"pool_keys_unmatched,omitempty"`
	PoolIndicesReplayed              *bool    `json:"pool_indices_replayed,omitempty"`
	PoolCoreIndicesStored            *int     `json:"pool_core_indices_stored,omitempty"`
	HDKeypoolCoreIndex               any      `json:"hd_keypool_core_index,omitempty"`
	PoolUnmatchedHint                string   `json:"pool_unmatched_hint,omitempty"`
	PoolReplayScanCap                int      `json:"pool_replay_scan_cap,omitempty"`
	WalletIndexHeight                *int64   `json:"wallet_index_height,omitempty"`
	ChainActiveHeight                *int64   `json:"chain_active_height,omitempty"`
	NeedsRescan                      *bool    `json:"needs_rescan,omitempty"`
	WalletScanIndexOK                *bool    `json:"wallet_scan_index_ok,omitempty"`
	WalletHistoryFastPath            *bool    `json:"wallet_history_fast_path,omitempty"`
	WalletListTransactionsUtxoWalk   *bool    `json:"wallet_listtransactions_utxo_walk,omitempty"`
	WalletListTransactionsScanPending *bool   `json:"wallet_listtransactions_scan_pending,omitempty"`
	WalletScanning                   bool     `json:"wallet_scanning,omitempty"`
	WalletHistoryDeferred            bool     `json:"wallet_history_deferred,omitempty"`
	WalletHistoryDeferReason         string   `json:"wallet_history_defer_reason,omitempty"`
	Notes                            []string `json:"notes,omitempty"`
	Issues                           []string `json:"issues,omitempty"`
	Warnings                         []string `json:"warnings,omitempty"`
	Hint                             string   `json:"hint,omitempty"`
}

const coreWalletProbeLabel = "dogego-probe"

// coreKeypoolRefillThreshold matches wallet.keypoolRefillThreshold (half of default 100).
const coreKeypoolRefillThreshold = 50

// coreWalletListTransactionsMaxMs is the Milestone E wallet history latency gate (40 rows, light path).
const coreWalletListTransactionsMaxMs = 3000

// ProbeCoreWallet runs getwalletinfo / getbalance / getnewaddress / validateaddress / address book / setlabel when wallet RPC is live.
func ProbeCoreWallet(invoke func(string, []json.RawMessage) map[string]interface{}) CoreWalletProbeResult {
	out := CoreWalletProbeResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Hint:      "Milestone E wallet basics - mirrors scripts/core_wallet_workflow.ps1. When DOGEGO_WALLET_DAT is set, includes dogego_probewalletdat (pool metadata, pool_indices_replayed on import with deep scan up to 2000 BIP44 indices via wallet/pool_replay.go, keypool hint, hd_keypool_core_index when stored). getwalletinfo may include wallet_index_height / needs_rescan / pq_commitments_enabled; use Settings → Wallet or setwalletflag pq_commitments true for FLC1 OP_RETURN on sends. Use rescan RPC or POST /api/wallet/rescan after upgrade to backfill fee and hex metadata. PSBT round-trip (walletcreatefundedpsbt + walletprocesspsbt) runs when wallet is unlocked with spendable balance; funded PSBTs should include BIP32 deriv paths for hardware signers.",
	}
	if invoke == nil {
		out.Skipped = true
		out.Reason = "RPC not ready"
		return out
	}
	wResp := invoke("getwalletinfo", nil)
	if errObj, ok := wResp["error"].(map[string]interface{}); ok && errObj != nil {
		msg := strings.ToLower(fmt.Sprint(errObj["message"]))
		code, _ := errObj["code"].(float64)
		if code == -1 || code == -32601 || strings.Contains(msg, "not implemented") || strings.Contains(msg, "not enabled") {
			out.Skipped = true
			out.Reason = "wallet not enabled"
			return out
		}
		out.Issues = append(out.Issues, "getwalletinfo_failed")
		out.OK = false
		return out
	}
	if wResp["result"] != nil {
		out.Wallet = wResp["result"]
		if m, ok := wResp["result"].(map[string]interface{}); ok {
			if v, ok := m["spendable_utxo_count"]; ok {
				switch n := v.(type) {
				case float64:
					i := int(n)
					out.SpendableUtxoCount = &i
				case int:
					i := n
					out.SpendableUtxoCount = &i
				}
			}
			if n := probeJSONInt(m["pool_core_indices_stored"]); n > 0 {
				out.PoolCoreIndicesStored = &n
			}
			if v, ok := m["hd_keypool_core_index"]; ok && v != nil {
				out.HDKeypoolCoreIndex = v
			}
			out.WalletIndexHeight = probeJSONInt64(m["wallet_index_height"])
			out.ChainActiveHeight = probeJSONInt64(m["chain_active_height"])
			if needs, _ := m["needs_rescan"].(bool); needs {
				t := true
				out.NeedsRescan = &t
			}
			if scanOK, ok := m["dogego_wallet_scan_index_ok"].(bool); ok {
				out.WalletScanIndexOK = &scanOK
			}
			if fastPath, ok := m["dogego_wallet_history_fast_path"].(bool); ok {
				out.WalletHistoryFastPath = &fastPath
			}
			if utxoWalk, ok := m["dogego_wallet_listtransactions_utxo_walk"].(bool); ok {
				out.WalletListTransactionsUtxoWalk = &utxoWalk
			}
			if scanPending, ok := m["dogego_wallet_listtransactions_scan_pending"].(bool); ok {
				out.WalletListTransactionsScanPending = &scanPending
			}
			if _, ok := m["scanning"]; ok {
				out.WalletScanning = true
			}
			if out.WalletScanning {
				out.Notes = append(out.Notes, "wallet_rescan_in_progress")
			}
			if out.WalletScanning && out.WalletListTransactionsUtxoWalk != nil && *out.WalletListTransactionsUtxoWalk {
				out.Notes = append(out.Notes, "wallet_scan_building_index: listtransactions may walk UTXOs until rescan populates wallet.db receive rows")
			}
			if out.NeedsRescan != nil && *out.NeedsRescan {
				if out.WalletHistoryFastPath != nil && *out.WalletHistoryFastPath {
					out.Notes = append(out.Notes, "wallet_index_lags_chain: rescan backfills fee_koinu and tx hex; listtransactions already uses partial scan index")
				} else {
					out.Notes = append(out.Notes, "wallet_index_lags_chain: use rescan or POST /api/wallet/rescan to backfill fee_koinu and tx hex; listtransactions may walk all UTXOs until indexed")
				}
			}
			if out.WalletScanIndexOK != nil && *out.WalletScanIndexOK {
				out.Notes = append(out.Notes, "wallet_scan_index_ok: listtransactions uses wallet.db history (no per-UTXO receive walk)")
			} else if out.WalletHistoryFastPath != nil && *out.WalletHistoryFastPath {
				out.Notes = append(out.Notes, "wallet_history_fast_path: listtransactions skips UTXO receive walk (partial scan index)")
			} else if out.WalletListTransactionsUtxoWalk != nil && *out.WalletListTransactionsUtxoWalk {
				if out.SpendableUtxoCount != nil && *out.SpendableUtxoCount > 64 {
					out.Notes = append(out.Notes, "wallet_listtransactions_utxo_walk: listtransactions walks UTXO cache for receive rows; rescan recommended for solo miners with many coinbases")
				} else {
					out.Notes = append(out.Notes, "wallet_listtransactions_utxo_walk: listtransactions walks UTXO cache for receive rows until wallet.db scan index exists")
				}
			}
			if configured, _ := m["signer_cmd_configured"].(bool); configured {
				out.SignerCmdConfigured = true
			}
			if enabled, ok := m["pq_commitments_enabled"].(bool); ok {
				out.PqCommitmentsEnabled = &enabled
				out.PqCommitmentsOK = enabled
				if !enabled {
					out.Notes = append(out.Notes, "pq_commitments_disabled: enable under Settings → Wallet or setwalletflag pq_commitments true")
				}
			} else {
				t := true
				out.PqCommitmentsEnabled = &t
				out.PqCommitmentsOK = true
			}
		}
	}
	if balResp := invoke("getbalance", nil); rpcInvokeFailed(balResp) {
		out.Warnings = append(out.Warnings, "getbalance_failed")
	} else if balResp["result"] != nil {
		out.Balance = balResp["result"]
	}
	addr := ""
	if addrResp := invoke("getnewaddress", mustRawJSON(`["receive"]`)); !rpcInvokeFailed(addrResp) {
		addr = rpcResultString(addrResp["result"])
	}
	if addr == "" {
		if addrResp := invoke("getaccountaddress", mustRawJSON(`[""]`)); !rpcInvokeFailed(addrResp) {
			addr = rpcResultString(addrResp["result"])
		} else {
			out.Warnings = append(out.Warnings, "getaddress_failed")
		}
	}
	out.Address = addr
	probeKeypoolTopupAfterGetNewAddress(invoke, addr, &out)
	if addr != "" {
		valResp := invoke("validateaddress", mustRawJSON(fmt.Sprintf(`[%q]`, addr)))
		if rpcInvokeFailed(valResp) {
			out.Warnings = append(out.Warnings, "validateaddress_failed")
		} else if m, ok := valResp["result"].(map[string]interface{}); ok {
			if valid, _ := m["isvalid"].(bool); !valid {
				out.Issues = append(out.Issues, "validateaddress_invalid")
			}
		}
	}
	if listResp := invoke("dogego_listwalletaddresses", nil); rpcInvokeFailed(listResp) {
		out.Warnings = append(out.Warnings, "dogego_listwalletaddresses_failed")
	} else {
		n := addressBookCount(listResp["result"])
		out.AddressBookCount = &n
		keypoolN, corePoolN := addressBookKeypoolStats(listResp["result"])
		nodeTipN := addressBookNodeTipCount(listResp["result"])
		out.AddressBookKeypoolCount = &keypoolN
		out.AddressBookNodeTipCount = &nodeTipN
		out.AddressBookCorePoolIndicesStored = &corePoolN
		if n == 0 && addr != "" {
			out.Notes = append(out.Notes, "address_book_pending_new_address")
		} else if n == 0 {
			out.Warnings = append(out.Warnings, "dogego_listwalletaddresses_empty")
		}
		probeKeypoolAddressRPCs(invoke, listResp["result"], &out)
		probeNodeTipAddressRPCs(invoke, listResp["result"], &out)
		if out.AddressBookCorePoolIndicesStored != nil && out.PoolCoreIndicesStored != nil &&
			*out.AddressBookCorePoolIndicesStored != *out.PoolCoreIndicesStored {
			out.Issues = append(out.Issues, "pool_core_indices_count_mismatch")
		}
	}
	if addr != "" && len(out.Issues) == 0 {
		addrParam, _ := json.Marshal(addr)
		labelParam, _ := json.Marshal(coreWalletProbeLabel)
		emptyParam, _ := json.Marshal("")
		if rpcInvokeFailed(invoke("setlabel", []json.RawMessage{addrParam, labelParam})) {
			out.Warnings = append(out.Warnings, "setlabel_failed")
		} else {
			byLabelParam, _ := json.Marshal(coreWalletProbeLabel)
			gResp := invoke("getaddressesbylabel", []json.RawMessage{byLabelParam})
			if rpcInvokeFailed(gResp) {
				out.Warnings = append(out.Warnings, "getaddressesbylabel_failed")
			} else if !addressInLabelResult(gResp["result"], addr) {
				out.Issues = append(out.Issues, "setlabel_roundtrip_mismatch")
			} else {
				out.LabelRoundTripOK = true
				if listResp := invoke("listlabels", nil); rpcInvokeFailed(listResp) {
					out.Warnings = append(out.Warnings, "listlabels_failed")
				} else if !labelInList(listResp["result"], coreWalletProbeLabel) {
					out.Issues = append(out.Issues, "listlabels_roundtrip_mismatch")
				} else {
					out.LabelListOK = true
				}
			}
			_ = invoke("setlabel", []json.RawMessage{addrParam, emptyParam})
		}
	}
	if enumResp := invoke("enumeratesigners", nil); rpcInvokeFailed(enumResp) {
		out.Warnings = append(out.Warnings, "enumeratesigners_failed")
		if out.SignerCmdConfigured {
			out.Notes = append(out.Notes, "signer_cmd configured but enumeratesigners failed (check HWI path and device)")
		}
	} else {
		n := addressBookCount(enumResp["result"])
		out.SignerCount = &n
		out.EnumerateSignersOK = true
		if n > 0 {
			out.SignerConfigured = true
		} else if out.SignerCmdConfigured {
			out.Notes = append(out.Notes, "signer_cmd configured but enumeratesigners returned 0 devices")
		}
	}
	probeWalletPsbtRoundTrip(invoke, addr, &out)
	out.WalletHistoryDeferReason = walletTxHistoryDeferReasonFromProbeState(invoke, out)
	if out.WalletHistoryDeferReason != "" {
		out.WalletHistoryDeferred = true
		out.Notes = append(out.Notes, fmt.Sprintf(
			"wallet_history_deferred_%s: History tab and GET /api/wallet/txs defer listtransactions (same as dashboard)",
			out.WalletHistoryDeferReason,
		))
	}
	probeWalletTxMetadata(invoke, &out)
	probeWalletDatOptional(invoke, &out)
	applyHardwarePsbtHint(&out)
	out.OK = len(out.Issues) == 0
	return out
}

func applyHardwarePsbtHint(out *CoreWalletProbeResult) {
	if out == nil || !out.SignerCmdConfigured || !out.PsbtCreateFundedOK {
		return
	}
	if out.PsbtRoundTripOK {
		if out.PsbtBIP32DerivOK {
			out.HardwarePsbtHint = "local keys signed funded PSBT; HWI signpsbt not exercised by probe (BIP32 paths attached)"
		} else {
			out.HardwarePsbtHint = "local keys signed funded PSBT; HWI signpsbt not exercised by probe"
		}
		return
	}
	if out.SignerConfigured {
		out.HardwarePsbtHint = "device listed; process incomplete - check HWI signpsbt or immature inputs"
	} else {
		out.HardwarePsbtHint = "connect HWI device; funded PSBT has BIP32 deriv paths for external signing"
	}
}

func probeWalletPsbtRoundTrip(invoke func(string, []json.RawMessage) map[string]interface{}, dest string, out *CoreWalletProbeResult) {
	if invoke == nil || out == nil {
		return
	}
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return
	}
	outputs, err := json.Marshal(map[string]float64{dest: 0.001})
	if err != nil {
		return
	}
	createResp := invoke("walletcreatefundedpsbt", []json.RawMessage{
		json.RawMessage(`[]`),
		outputs,
	})
	if rpcInvokeFailed(createResp) {
		code, msg := rpcErrorCodeMessage(createResp)
		if code == -6 || strings.Contains(strings.ToLower(msg), "insufficient") {
			out.Notes = append(out.Notes, "psbt_roundtrip_skipped: insufficient mature balance for 0.001 DOGE probe")
			return
		}
		if code == -13 || strings.Contains(strings.ToLower(msg), "unlock") {
			out.Notes = append(out.Notes, "psbt_roundtrip_skipped: wallet locked (unlock for PSBT probe)")
			return
		}
		out.Warnings = append(out.Warnings, "walletcreatefundedpsbt_failed")
		return
	}
	res, ok := createResp["result"].(map[string]interface{})
	if !ok || res == nil {
		out.Warnings = append(out.Warnings, "walletcreatefundedpsbt_empty")
		return
	}
	psbtB64, _ := res["psbt"].(string)
	if strings.TrimSpace(psbtB64) == "" {
		out.Warnings = append(out.Warnings, "walletcreatefundedpsbt_no_psbt")
		return
	}
	out.PsbtCreateFundedOK = true
	if psbtHasBIP32Derivations(invoke, psbtB64) {
		out.PsbtBIP32DerivOK = true
	} else {
		out.Warnings = append(out.Warnings, "psbt_bip32_deriv_missing")
	}
	psbtJ, _ := json.Marshal(psbtB64)
	procResp := invoke("walletprocesspsbt", []json.RawMessage{
		psbtJ,
		json.RawMessage(`true`),
		json.RawMessage(`"ALL"`),
		json.RawMessage(`true`),
		json.RawMessage(`true`),
	})
	if rpcInvokeFailed(procResp) {
		code, msg := rpcErrorCodeMessage(procResp)
		if code == -13 || strings.Contains(strings.ToLower(msg), "unlock") {
			out.Notes = append(out.Notes, "psbt_process_skipped: wallet locked after create")
			return
		}
		out.Warnings = append(out.Warnings, "walletprocesspsbt_failed")
		return
	}
	proc, ok := procResp["result"].(map[string]interface{})
	if !ok || proc == nil {
		out.Warnings = append(out.Warnings, "walletprocesspsbt_empty")
		return
	}
	if complete, _ := proc["complete"].(bool); complete {
		out.PsbtProcessComplete = true
		out.PsbtRoundTripOK = true
	} else {
		out.Notes = append(out.Notes, "psbt_process_incomplete: check signer_cmd or immature inputs")
	}
	if out.SignerCmdConfigured && out.PsbtCreateFundedOK && !out.PsbtRoundTripOK {
		out.Notes = append(out.Notes, "psbt_probe_uses_local_signing: external signer_cmd HWI path not exercised by round-trip probe")
	}
}

func probeKeypoolTopupAfterGetNewAddress(invoke func(string, []json.RawMessage) map[string]interface{}, addr string, out *CoreWalletProbeResult) {
	if invoke == nil || out == nil || strings.TrimSpace(addr) == "" {
		return
	}
	wi := invoke("getwalletinfo", nil)
	if rpcInvokeFailed(wi) {
		return
	}
	m, ok := wi["result"].(map[string]interface{})
	if !ok || m == nil {
		return
	}
	if format, _ := m["format"].(string); format != "hd" {
		return
	}
	n := probeJSONInt(m["keypoolsize"])
	if n > 0 {
		out.KeypoolSizeAfter = &n
	}
	if n >= coreKeypoolRefillThreshold {
		out.KeypoolTopupOK = true
	} else {
		out.Warnings = append(out.Warnings, "keypool_below_threshold")
	}
}

func psbtHasBIP32Derivations(invoke func(string, []json.RawMessage) map[string]interface{}, psbtB64 string) bool {
	psbtB64 = strings.TrimSpace(psbtB64)
	if psbtB64 == "" || invoke == nil {
		return false
	}
	psbtJ, err := json.Marshal(psbtB64)
	if err != nil {
		return false
	}
	dec := invoke("decodepsbt", []json.RawMessage{psbtJ})
	if rpcInvokeFailed(dec) {
		return false
	}
	m, ok := dec["result"].(map[string]interface{})
	if !ok || m == nil {
		return false
	}
	return psbtSectionHasBIP32Derivs(m["inputs"]) || psbtSectionHasBIP32Derivs(m["outputs"])
}

func psbtSectionHasBIP32Derivs(section interface{}) bool {
	arr, ok := section.([]interface{})
	if !ok {
		return false
	}
	for _, item := range arr {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		switch derivs := row["bip32_derivs"].(type) {
		case []interface{}:
			if len(derivs) > 0 {
				return true
			}
		}
	}
	return false
}

func rpcErrorCodeMessage(resp map[string]interface{}) (int, string) {
	if resp == nil {
		return 0, ""
	}
	errObj, ok := resp["error"].(map[string]interface{})
	if !ok || errObj == nil {
		return 0, ""
	}
	code := 0
	switch c := errObj["code"].(type) {
	case float64:
		code = int(c)
	case int:
		code = c
	}
	msg, _ := errObj["message"].(string)
	return code, msg
}

func probeWalletTxMetadata(invoke func(string, []json.RawMessage) map[string]interface{}, out *CoreWalletProbeResult) {
	if invoke == nil || out == nil {
		return
	}
	if out.WalletHistoryDeferred {
		out.Notes = append(out.Notes, "listtransactions_skipped: wallet_history_deferred")
		return
	}
	start := time.Now()
	listResp := invoke("listtransactions", []json.RawMessage{
		json.RawMessage(`"*"`), json.RawMessage(`40`),
	})
	out.WalletListTransactionsMs = int(time.Since(start).Milliseconds())
	out.WalletListTransactionsOK = out.WalletListTransactionsMs < coreWalletListTransactionsMaxMs
	if !out.WalletListTransactionsOK {
		out.Notes = append(out.Notes, fmt.Sprintf(
			"listtransactions_slow: %dms (threshold %dms; History tab uses same RPC bridge)",
			out.WalletListTransactionsMs, coreWalletListTransactionsMaxMs,
		))
	}
	if rpcInvokeFailed(listResp) {
		return
	}
	arr, ok := listResp["result"].([]interface{})
	if !ok || len(arr) == 0 {
		out.Notes = append(out.Notes, "wallet_tx_metadata_skipped: no listtransactions rows")
		return
	}
	var txid string
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		cat := strings.ToLower(strFromAny(m["category"]))
		if cat != "send" {
			continue
		}
		if conf := probeJSONInt(m["confirmations"]); conf < 1 {
			continue
		}
		rowTxid := strings.TrimSpace(strFromAny(m["txid"]))
		if rowTxid == "" {
			continue
		}
		if txid == "" {
			txid = rowTxid
		}
		if out.PqCommitmentsOK && !out.WalletPqSendOK {
			if kind, tag := walletPqMetaFromGetTransaction(invoke, rowTxid); kind == "sent_pq" {
				out.WalletPqSendOK = true
				out.WalletPqTag = tag
			}
		}
	}
	if out.PqCommitmentsOK && !out.WalletPqSendOK {
		hasSend := txid != ""
		if !hasSend {
			out.Notes = append(out.Notes, "wallet_pq_send_skipped: no confirmed send rows")
		} else {
			out.Notes = append(out.Notes, "wallet_pq_send_pending: pq_commitments on but no PQ-tagged send in history yet")
		}
	}
	if txid == "" {
		out.Notes = append(out.Notes, "wallet_tx_metadata_skipped: no confirmed send rows")
		return
	}
	txidJ, _ := json.Marshal(txid)
	gtResp := invoke("gettransaction", []json.RawMessage{txidJ})
	if rpcInvokeFailed(gtResp) {
		out.Warnings = append(out.Warnings, "gettransaction_metadata_failed")
		return
	}
	gm, ok := gtResp["result"].(map[string]interface{})
	if !ok || gm == nil {
		out.Warnings = append(out.Warnings, "gettransaction_metadata_empty")
		return
	}
	if hx := strings.TrimSpace(strFromAny(gm["hex"])); hx != "" {
		out.WalletTxHexOK = true
	}
	if fee, ok := gm["fee"].(float64); ok && fee < 0 {
		out.WalletTxFeeOK = true
	} else if feeNum, ok := gm["fee"].(json.Number); ok {
		if f, err := feeNum.Float64(); err == nil && f < 0 {
			out.WalletTxFeeOK = true
		}
	}
	if !out.WalletTxHexOK {
		out.Notes = append(out.Notes, "wallet_tx_hex_missing: run POST /api/wallet/rescan or enable tx_index_embed_tx for block load")
	}
}

func walletPqMetaFromGetTransaction(invoke func(string, []json.RawMessage) map[string]interface{}, txid string) (kind, pqTag string) {
	if invoke == nil || txid == "" {
		return "", ""
	}
	txidJ, err := json.Marshal(txid)
	if err != nil {
		return "", ""
	}
	gtResp := invoke("gettransaction", []json.RawMessage{txidJ})
	if rpcInvokeFailed(gtResp) {
		return "", ""
	}
	gm, ok := gtResp["result"].(map[string]interface{})
	if !ok || gm == nil {
		return "", ""
	}
	hx := strings.TrimSpace(strFromAny(gm["hex"]))
	return walletSendPQMetaFromHex(hx)
}

func probeWalletDatOptional(invoke func(string, []json.RawMessage) map[string]interface{}, out *CoreWalletProbeResult) {
	if out == nil || invoke == nil {
		return
	}
	path := strings.TrimSpace(os.Getenv("DOGEGO_WALLET_DAT"))
	if path == "" {
		return
	}
	out.WalletDatPath = path
	if _, err := os.Stat(path); err != nil {
		out.Warnings = append(out.Warnings, "walletdat_probe_path_missing")
		return
	}
	pathJ, err := json.Marshal(path)
	if err != nil {
		return
	}
	resp := invoke("dogego_probewalletdat", []json.RawMessage{pathJ})
	if rpcInvokeFailed(resp) {
		out.Warnings = append(out.Warnings, "walletdat_probe_failed")
		return
	}
	if resp["result"] == nil {
		out.Warnings = append(out.Warnings, "walletdat_probe_empty")
		return
	}
	out.WalletDatProbe = resp["result"]
	m, _ := resp["result"].(map[string]interface{})
	if m == nil {
		return
	}
	if isBDB, _ := m["is_bdb"].(bool); !isBDB {
		out.Warnings = append(out.Warnings, "walletdat_probe_not_bdb")
		return
	}
	needsPass, _ := m["needs_passphrase"].(bool)
	canImport, _ := m["can_import"].(bool)
	encrypted, _ := m["encrypted"].(bool)
	switch {
	case needsPass && !canImport:
		out.Warnings = append(out.Warnings, "walletdat_probe_encrypted_no_master_key")
	case !canImport && !encrypted:
		out.Warnings = append(out.Warnings, "walletdat_probe_cannot_import")
	}
	if probeJSONInt(m["pool_count"]) > 0 {
		out.PoolReplayScanCap = wallet.PoolReplayScanCap
		if n := probeJSONInt(m["pool_keys_unmatched"]); n > 0 {
			out.PoolKeysUnmatched = &n
			out.Notes = append(out.Notes, "walletdat_probe_pool_keys_unmatched")
		}
		if hint := strings.TrimSpace(fmt.Sprint(m["pool_unmatched_hint"])); hint != "" && hint != "<nil>" {
			out.PoolUnmatchedHint = hint
		}
		if r, ok := m["pool_indices_replayed"].(bool); ok {
			out.PoolIndicesReplayed = &r
		}
	}
	if out.PoolCoreIndicesStored == nil {
		if n := probeJSONInt(m["pool_core_indices_stored"]); n > 0 {
			out.PoolCoreIndicesStored = &n
		}
	}
	if out.HDKeypoolCoreIndex == nil {
		if v, ok := m["hd_keypool_core_index"]; ok && v != nil {
			out.HDKeypoolCoreIndex = v
		}
	}
}

func probeJSONInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}

func addressBookKeypoolStats(v any) (keypoolCount, corePoolIndexCount int) {
	switch arr := v.(type) {
	case []interface{}:
		for _, row := range arr {
			kp, cp := addressBookRowKeypoolStats(row)
			if kp {
				keypoolCount++
			}
			if cp {
				corePoolIndexCount++
			}
		}
	default:
		rv := reflect.ValueOf(v)
		if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
			for i := 0; i < rv.Len(); i++ {
				kp, cp := addressBookRowKeypoolStats(rv.Index(i).Interface())
				if kp {
					keypoolCount++
				}
				if cp {
					corePoolIndexCount++
				}
			}
		}
	}
	return keypoolCount, corePoolIndexCount
}

func addressBookRowKeypoolStats(row any) (iskeypool, hasCoreIndex bool) {
	m, ok := row.(map[string]interface{})
	if !ok {
		return false, false
	}
	if b, ok := m["iskeypool"].(bool); ok && b {
		iskeypool = true
	}
	if v, ok := m["hd_keypool_core_index"]; ok && v != nil {
		hasCoreIndex = true
	}
	return iskeypool, hasCoreIndex
}

func addressBookNodeTipCount(v any) int {
	n := 0
	switch arr := v.(type) {
	case []interface{}:
		for _, row := range arr {
			if addressBookRowIsNodeTip(row) {
				n++
			}
		}
	default:
		rv := reflect.ValueOf(v)
		if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
			for i := 0; i < rv.Len(); i++ {
				if addressBookRowIsNodeTip(rv.Index(i).Interface()) {
					n++
				}
			}
		}
	}
	return n
}

func addressBookRowIsNodeTip(row any) bool {
	m, ok := row.(map[string]interface{})
	if !ok {
		return false
	}
	b, ok := m["isnodetip"].(bool)
	return ok && b
}

func firstKeypoolAddressRow(v any) (addr string, coreIndex *int64) {
	switch arr := v.(type) {
	case []interface{}:
		for _, row := range arr {
			if a, idx, ok := keypoolAddressFromRow(row); ok {
				return a, idx
			}
		}
	default:
		rv := reflect.ValueOf(v)
		if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
			for i := 0; i < rv.Len(); i++ {
				if a, idx, ok := keypoolAddressFromRow(rv.Index(i).Interface()); ok {
					return a, idx
				}
			}
		}
	}
	return "", nil
}

func keypoolAddressFromRow(row any) (addr string, coreIndex *int64, ok bool) {
	m, mapOK := row.(map[string]interface{})
	if !mapOK {
		return "", nil, false
	}
	isKP, _ := m["iskeypool"].(bool)
	if !isKP {
		return "", nil, false
	}
	addr = strings.TrimSpace(rpcResultString(m["address"]))
	if addr == "" {
		return "", nil, false
	}
	if v, exists := m["hd_keypool_core_index"]; exists && v != nil {
		switch n := v.(type) {
		case float64:
			i := int64(n)
			coreIndex = &i
		case int64:
			i := n
			coreIndex = &i
		case int:
			i := int64(n)
			coreIndex = &i
		}
	}
	return addr, coreIndex, true
}

func probeKeypoolAddressRPCs(invoke func(string, []json.RawMessage) map[string]interface{}, listResult any, out *CoreWalletProbeResult) {
	if invoke == nil || out == nil {
		return
	}
	addr, wantCoreIdx := firstKeypoolAddressRow(listResult)
	if addr == "" {
		return
	}
	addrParam, err := json.Marshal(addr)
	if err != nil {
		return
	}
	valResp := invoke("validateaddress", []json.RawMessage{addrParam})
	if rpcInvokeFailed(valResp) {
		out.Warnings = append(out.Warnings, "keypool_validateaddress_failed")
	} else if m, ok := valResp["result"].(map[string]interface{}); ok {
		if valid, _ := m["isvalid"].(bool); !valid {
			out.Issues = append(out.Issues, "keypool_validateaddress_invalid")
		} else if isKP, _ := m["iskeypool"].(bool); !isKP {
			out.Issues = append(out.Issues, "keypool_validateaddress_mismatch")
		} else {
			out.KeypoolValidateAddressOK = true
			if wantCoreIdx != nil {
				if got := probeJSONInt64(m["hd_keypool_core_index"]); got == nil || *got != *wantCoreIdx {
					out.Issues = append(out.Issues, "keypool_validateaddress_core_index_mismatch")
					out.KeypoolValidateAddressOK = false
				}
			}
		}
	}
	infoResp := invoke("getaddressinfo", []json.RawMessage{addrParam})
	if rpcInvokeFailed(infoResp) {
		out.Warnings = append(out.Warnings, "keypool_getaddressinfo_failed")
	} else if m, ok := infoResp["result"].(map[string]interface{}); ok {
		if isKP, _ := m["iskeypool"].(bool); !isKP {
			out.Issues = append(out.Issues, "keypool_getaddressinfo_mismatch")
		} else {
			out.KeypoolGetAddressInfoOK = true
			if wantCoreIdx != nil {
				if got := probeJSONInt64(m["hd_keypool_core_index"]); got == nil || *got != *wantCoreIdx {
					out.Issues = append(out.Issues, "keypool_getaddressinfo_core_index_mismatch")
					out.KeypoolGetAddressInfoOK = false
				}
			}
		}
	}
}

func firstNodeTipAddressRow(v any) string {
	switch arr := v.(type) {
	case []interface{}:
		for _, row := range arr {
			if addr := nodeTipAddressFromRow(row); addr != "" {
				return addr
			}
		}
	default:
		rv := reflect.ValueOf(v)
		if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
			for i := 0; i < rv.Len(); i++ {
				if addr := nodeTipAddressFromRow(rv.Index(i).Interface()); addr != "" {
					return addr
				}
			}
		}
	}
	return ""
}

func nodeTipAddressFromRow(row any) string {
	m, ok := row.(map[string]interface{})
	if !ok {
		return ""
	}
	isNT, _ := m["isnodetip"].(bool)
	if !isNT {
		return ""
	}
	return strings.TrimSpace(rpcResultString(m["address"]))
}

func probeNodeTipAddressRPCs(invoke func(string, []json.RawMessage) map[string]interface{}, listResult any, out *CoreWalletProbeResult) {
	if invoke == nil || out == nil {
		return
	}
	addr := firstNodeTipAddressRow(listResult)
	if addr == "" {
		return
	}
	addrParam, err := json.Marshal(addr)
	if err != nil {
		return
	}
	valResp := invoke("validateaddress", []json.RawMessage{addrParam})
	if rpcInvokeFailed(valResp) {
		out.Warnings = append(out.Warnings, "nodetip_validateaddress_failed")
	} else if m, ok := valResp["result"].(map[string]interface{}); ok {
		if valid, _ := m["isvalid"].(bool); !valid {
			out.Issues = append(out.Issues, "nodetip_validateaddress_invalid")
		} else if isNT, _ := m["isnodetip"].(bool); !isNT {
			out.Issues = append(out.Issues, "nodetip_validateaddress_mismatch")
		} else {
			out.NodeTipValidateAddressOK = true
		}
	}
	infoResp := invoke("getaddressinfo", []json.RawMessage{addrParam})
	if rpcInvokeFailed(infoResp) {
		out.Warnings = append(out.Warnings, "nodetip_getaddressinfo_failed")
	} else if m, ok := infoResp["result"].(map[string]interface{}); ok {
		if isNT, _ := m["isnodetip"].(bool); !isNT {
			out.Issues = append(out.Issues, "nodetip_getaddressinfo_mismatch")
		} else {
			out.NodeTipGetAddressInfoOK = true
		}
	}
}

func probeJSONInt64(v any) *int64 {
	switch n := v.(type) {
	case float64:
		i := int64(n)
		return &i
	case int64:
		i := n
		return &i
	case int:
		i := int64(n)
		return &i
	default:
		return nil
	}
}

func addressBookCount(v any) int {
	switch arr := v.(type) {
	case []interface{}:
		return len(arr)
	default:
		rv := reflect.ValueOf(v)
		if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
			return rv.Len()
		}
		return 0
	}
}

func addressInLabelResult(v any, want string) bool {
	switch t := v.(type) {
	case map[string]interface{}:
		_, ok := t[want]
		return ok
	case []interface{}:
		for _, row := range t {
			if rpcResultString(row) == want {
				return true
			}
		}
	default:
		rv := reflect.ValueOf(v)
		if rv.IsValid() && rv.Kind() == reflect.Map && rv.Type().Key().Kind() == reflect.String {
			return rv.MapIndex(reflect.ValueOf(want)).IsValid()
		}
		if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
			for i := 0; i < rv.Len(); i++ {
				if rpcResultString(rv.Index(i).Interface()) == want {
					return true
				}
			}
		}
	}
	return false
}

func labelInList(v any, want string) bool {
	switch arr := v.(type) {
	case []interface{}:
		for _, row := range arr {
			if rpcResultString(row) == want {
				return true
			}
		}
	default:
		rv := reflect.ValueOf(v)
		if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
			for i := 0; i < rv.Len(); i++ {
				if rpcResultString(rv.Index(i).Interface()) == want {
					return true
				}
			}
		}
	}
	return false
}

func rpcInvokeFailed(resp map[string]interface{}) bool {
	if resp == nil {
		return true
	}
	errObj, ok := resp["error"].(map[string]interface{})
	return ok && errObj != nil
}

func rpcResultString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(v)
	}
}

func mustRawJSON(s string) []json.RawMessage {
	return []json.RawMessage{json.RawMessage(s)}
}
