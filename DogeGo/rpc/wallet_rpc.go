// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"dogego/chain"
	"dogego/mempool"
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

func rpcWalletAddress(paths *DataPaths) string {
	return rpcWalletDefaultAddress(paths)
}

func rpcWalletDefaultAddress(paths *DataPaths) string {
	if paths == nil || paths.WalletDefaultAddress == nil {
		if paths != nil && paths.WalletAddress != nil {
			return strings.TrimSpace(paths.WalletAddress())
		}
		return ""
	}
	return strings.TrimSpace(paths.WalletDefaultAddress())
}

func rpcWalletSpendScripts(paths *DataPaths) [][]byte {
	if paths == nil || paths.WalletSpendScripts == nil {
		if spk := walletPkScript(paths); len(spk) > 0 {
			return [][]byte{spk}
		}
		return nil
	}
	return paths.WalletSpendScripts()
}

func rpcWalletContainsAddress(paths *DataPaths, addr string) bool {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return false
	}
	if addr == rpcWalletDefaultAddress(paths) {
		return true
	}
	if paths != nil && paths.WalletContainsAddress != nil {
		return paths.WalletContainsAddress(addr)
	}
	return false
}

func rpcWalletWIF(paths *DataPaths) string {
	if paths == nil || paths.WalletWIF == nil {
		return ""
	}
	return strings.TrimSpace(paths.WalletWIF())
}

func rpcWalletWIFs(paths *DataPaths) []string {
	if paths == nil {
		return nil
	}
	if paths.WalletWIFs != nil {
		if w := paths.WalletWIFs(); len(w) > 0 {
			return w
		}
	}
	if w := rpcWalletWIF(paths); w != "" {
		return []string{w}
	}
	return nil
}

func walletPkScript(paths *DataPaths) []byte {
	if paths == nil || paths.WalletP2PKHScript == nil {
		return nil
	}
	return paths.WalletP2PKHScript()
}

// execGetNewAddress returns the built-in testnet wallet address (single-key "keypool").
func execGetNewAddress(chainName string, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var label string
	if len(params) >= 1 && strings.TrimSpace(string(params[0])) != "null" {
		if _, code, msg := parseRPCAccountLabel(params[0], "getnewaddress", "label"); code != 0 {
			return nil, code, msg
		}
		_ = json.Unmarshal(params[0], &label)
		label = strings.TrimSpace(label)
	}
	if len(params) == 2 && strings.TrimSpace(string(params[1])) != "null" {
		var addrType string
		if err := json.Unmarshal(params[1], &addrType); err != nil {
			return nil, -8, "getnewaddress: address_type must be a string"
		}
		addrType = strings.TrimSpace(strings.ToLower(addrType))
		if addrType != "" && addrType != "legacy" {
			return nil, -8, "getnewaddress: address type not supported"
		}
	}
	if paths == nil || paths.WalletNewAddress == nil {
		return nil, -1, "getnewaddress: wallet is not implemented in DogeGo"
	}
	addr, err := paths.WalletNewAddress()
	if err != nil {
		if code, msg := rpcWalletOpErr(err); code != 0 {
			if code == -13 {
				return nil, code, msg
			}
			return nil, code, "getnewaddress: "+msg
		}
		return nil, -1, "getnewaddress: "+err.Error()
	}
	if addr == "" {
		return nil, -1, "getnewaddress: wallet is not implemented in DogeGo"
	}
	if label != "" && paths.WalletSetLabel != nil {
		_ = paths.WalletSetLabel(addr, label)
	}
	return addr, 0, ""
}

// execGetRawChangeAddress returns the same address as getnewaddress for the built-in wallet.
func execGetRawChangeAddress(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	if len(params) == 1 && strings.TrimSpace(string(params[0])) != "null" {
		if _, code, msg := parseRPCAccountLabel(params[0], "getrawchangeaddress", "account"); code != 0 {
			return nil, code, msg
		}
	}
	if paths == nil || paths.WalletNewChangeAddress == nil {
		return nil, -1, "getrawchangeaddress: wallet is not implemented in DogeGo"
	}
	addr, err := paths.WalletNewChangeAddress()
	if err != nil {
		if code, msg := rpcWalletOpErr(err); code != 0 {
			if code == -13 {
				return nil, code, msg
			}
			return nil, code, "getrawchangeaddress: "+msg
		}
		return nil, -1, "getrawchangeaddress: "+err.Error()
	}
	if addr == "" {
		return nil, -1, "getrawchangeaddress: wallet is not implemented in DogeGo"
	}
	return addr, 0, ""
}

// execDumpPrivKey reveals the WIF for the built-in wallet address only.
func execDumpPrivKey(chainName string, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var addr string
	if err := json.Unmarshal(params[0], &addr); err != nil {
		return nil, -8, "dumpprivkey: address must be a string"
	}
	addr = strings.TrimSpace(addr)
	vis, _, _ := ValidateAddressString(chainName, addr)
	if ok, _ := vis["isvalid"].(bool); !ok {
		return nil, -5, "Invalid Dogecoin address"
	}
	if !rpcWalletContainsAddress(paths, addr) {
		return nil, -4, "Private key for address "+addr+" is not known"
	}
	if code, msg := rpcWalletRequireMainnetEncrypted(chainName, paths); code != 0 {
		return nil, code, msg
	}
	if code, msg := rpcWalletRequireUnlocked(paths); code != 0 {
		return nil, code, msg
	}
	var wif string
	if paths.WalletWIFForAddress != nil {
		w, err := paths.WalletWIFForAddress(addr)
		if err != nil {
			return nil, -4, "Private key for address "+addr+" is not known"
		}
		wif = w
	} else {
		wif = rpcWalletWIF(paths)
	}
	if wif == "" {
		return nil, -4, "Private key for address "+addr+" is not known"
	}
	if wif == "" {
		return nil, -4, "Private key for address " + addr + " is not known"
	}
	return wif, 0, ""
}

// execListUnspent lists P2PKH UTXOs paid to the built-in wallet from the node UTXO cache.
func execListUnspent(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, ix *store.TxIndex, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 5 {
		return nil, -32602, "Wrong number of arguments"
	}
	minConf := int64(1)
	maxConf := int64(9999999)
	if len(params) > 0 && strings.TrimSpace(string(params[0])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[0], &n); err != nil {
			return nil, -8, "listunspent: minconf must be a number"
		}
		mi, err := n.Int64()
		if err != nil || mi < 0 {
			return nil, -8, "listunspent: minconf out of range"
		}
		minConf = mi
	}
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[1], &n); err != nil {
			return nil, -8, "listunspent: maxconf must be a number"
		}
		ma, err := n.Int64()
		if err != nil || ma < 0 {
			return nil, -8, "listunspent: maxconf out of range"
		}
		maxConf = ma
	}
	var filterAddrs map[string]struct{}
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var arr []json.RawMessage
		if err := json.Unmarshal(params[2], &arr); err != nil {
			return nil, -8, "listunspent: addresses must be a JSON array"
		}
		filterAddrs = make(map[string]struct{})
		for _, elem := range arr {
			var addr string
			if err := json.Unmarshal(elem, &addr); err != nil {
				return nil, -8, "listunspent: Invalid parameter, expected string address"
			}
			addr = strings.TrimSpace(addr)
			vis, _, _ := ValidateAddressString(chainName, addr)
			isv, _ := vis["isvalid"].(bool)
			if !isv {
				return nil, -5, "Invalid Dogecoin address: " + addr
			}
			if _, dup := filterAddrs[addr]; dup {
				return nil, -8, "Invalid parameter, duplicated address: " + addr
			}
			filterAddrs[addr] = struct{}{}
		}
	}
	includeUnsafe := true
	if len(params) > 3 && strings.TrimSpace(string(params[3])) != "null" {
		var code int
		var msg string
		includeUnsafe, code, msg = parseRPCBoolOpt(params[3], true, "listunspent", "include_unsafe")
		if code != 0 {
			return nil, code, msg
		}
	}
	var queryOpts listUnspentQueryOpts
	if len(params) > 4 && strings.TrimSpace(string(params[4])) != "null" {
		var code int
		var msg string
		queryOpts, code, msg = parseListUnspentQueryOptions(params[4])
		if code != 0 {
			return nil, code, msg
		}
	}
	if rpcWalletAddress(paths) == "" && len(rpcWalletWatchScripts(paths)) == 0 {
		return []interface{}{}, 0, ""
	}
	matches, code, msg := walletUtxoMatches(paths, j, raw, chainName, minConf, 0)
	if code != 0 {
		return nil, code, msg
	}
	matches = filterListUnspentMatches(matches, queryOpts)
	coinbaseMaturity := walletCoinbaseMaturity(chainName, j, raw, paths)
	arr := make([]interface{}, 0, len(matches))
	enrichMeta := len(matches) <= 128
	for _, m := range matches {
		if filterAddrs != nil {
			if _, ok := filterAddrs[m.address]; !ok {
				continue
			}
		}
		if maxConf < 9999999 && m.confirmations > maxConf {
			continue
		}
		safe := walletUtxoIsSafe(m, coinbaseMaturity, ix, raw)
		if !includeUnsafe && !safe {
			continue
		}
		spendable := m.spendable
		if spendable && m.confirmations < coinbaseMaturity && walletUtxoImmatureCoinbase(m.row, ix, raw) {
			spendable = false
		}
		entry := map[string]interface{}{
			"txid":          m.row.TxID,
			"vout":          m.row.Vout,
			"address":       m.address,
			"label":         rpcWalletGetLabel(paths, m.address),
			"scriptPubKey":  hex.EncodeToString(m.row.PkScript),
			"amount":        float64(m.row.Value) / 1e8,
			"confirmations": m.confirmations,
			"spendable":     spendable,
			"solvable":      spendable,
			"safe":          safe,
			"iswatchonly":   !m.spendable,
		}
		if rpcWalletAvoidReuse(paths) && paths.WalletIsScriptReused != nil {
			entry["reused"] = paths.WalletIsScriptReused(m.row.PkScript)
		}
		if m.row.Height >= 0 {
			entry["height"] = m.row.Height
			if enrichMeta {
				if bh := walletBlockHashCached(j, m.row.Height); bh != "" {
					entry["blockhash"] = bh
				}
				if t := walletHeaderTimeCached(j, m.row.Height); t > 0 {
					entry["timereceived"] = t
				}
			}
		}
		arr = append(arr, entry)
	}
	return arr, 0, ""
}

// execGetBalance sums spendable wallet UTXOs (minconf, default 1; excludes immature coinbase).
func execGetBalance(paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, ix *store.TxIndex, chainName string, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 3 {
		return nil, -32602, "Wrong number of arguments"
	}
	if len(params) > 0 && strings.TrimSpace(string(params[0])) != "null" {
		var account string
		if err := json.Unmarshal(params[0], &account); err != nil {
			return nil, -8, "getbalance: account must be a string"
		}
	}
	minConf := int64(1)
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[1], &n); err != nil {
			return nil, -8, "getbalance: minconf must be a number"
		}
		mi, err := n.Int64()
		if err != nil || mi < 0 {
			return nil, -8, "getbalance: minconf out of range"
		}
		minConf = mi
	}
	includeWatchonly := false
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var code int
		var msg string
		includeWatchonly, code, msg = parseRPCBoolOpt(params[2], false, "getbalance", "include_watchonly")
		if code != 0 {
			return nil, code, msg
		}
	}
	matches, code, msg := walletUtxoMatches(paths, j, raw, chainName, minConf, 0)
	if code != 0 {
		return nil, code, msg
	}
	var sum int64
	for _, m := range matches {
		if !m.spendable && !(includeWatchonly && !m.spendable) {
			continue
		}
		if m.spendable && !utxoRowSpendableForFund(paths, j, raw, ix, m.row, chainName) {
			continue
		}
		sum += m.row.Value
	}
	return float64(sum) / 1e8, 0, ""
}

// execGetWalletInfo returns balances from the UTXO cache for the built-in wallet.
func execGetWalletInfo(paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, ix *store.TxIndex, chainName string, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 0 {
		return nil, -32602, "Wrong number of arguments"
	}
	if rpcWalletAddress(paths) == "" {
		return map[string]interface{}{
			"walletversion":       0,
			"balance":             0.0,
			"unconfirmed_balance":   0.0,
			"immature_balance":    0.0,
			"txcount":             0,
			"keypoololdest":       0,
			"keypoolsize":         0,
			"unlocked_until":      0,
			"paytxfee":            0.0,
		}, 0, ""
	}
	coinbaseMaturity := walletCoinbaseMaturity(chainName, j, raw, paths)
	matches, code, msg := walletUtxoMatches(paths, j, raw, chainName, 0, 0)
	if code != 0 {
		return nil, code, msg
	}
	var confirmed, immature int64
	spendableUTXOs := 0
	for _, m := range matches {
		if !m.spendable {
			continue
		}
		if m.confirmations >= coinbaseMaturity {
			confirmed += m.row.Value
			spendableUTXOs++
		} else if m.confirmations > 0 && walletUtxoImmatureCoinbase(m.row, ix, raw) {
			immature += m.row.Value
		} else if m.confirmations > 0 {
			confirmed += m.row.Value
			spendableUTXOs++
		}
	}
	walletName := "wallet"
	if paths != nil && paths.WalletPath != nil {
		if p := strings.TrimSpace(paths.WalletPath()); p != "" {
			walletName = filepathBase(p)
		}
	}
	walletFormat := "dogego-builtin"
	keypoolSize := 1
	if paths != nil && paths.WalletHDFormat != nil {
		if f := strings.TrimSpace(paths.WalletHDFormat()); f == "hd" {
			walletFormat = "hd"
		}
	}
	if paths != nil && paths.WalletKeypoolSize != nil {
		if n := paths.WalletKeypoolSize(); n > 0 {
			keypoolSize = n
		}
	}
	encrypted := rpcWalletIsEncrypted(paths)
	note := "built-in wallet; BIP44 m/44'/3'/0'/0/n receive and …/1/n change when format is hd"
	if walletFormat != "hd" {
		note = "built-in single-address wallet; addmultisigaddress imports watch-only P2SH"
	}
	if encrypted {
		note += "; use walletpassphrase before spending"
	}
	privateKeys := !encrypted || (paths != nil && paths.WalletIsUnlocked != nil && paths.WalletIsUnlocked())
	info := map[string]interface{}{
		"walletname":            walletName,
		"walletversion":         60000,
		"format":                walletFormat,
		"encrypted":             encrypted,
		"private_keys_enabled":  privateKeys,
		"balance":               float64(confirmed) / 1e8,
		"unconfirmed_balance": float64(walletMempoolNetKoinu(chainName, paths, pool)) / 1e8,
		"immature_balance":    float64(immature) / 1e8,
		"txcount":              len(matches),
		"spendable_utxo_count": spendableUTXOs,
		"keypoololdest":       0,
		"keypoolsize":         keypoolSize,
		"unlocked_until":      rpcWalletUnlockUntil(paths),
		"paytxfee":            rpcWalletPayTxFee(paths),
		"dogego_note":         note,
	}
	info["avoid_reuse"] = rpcWalletAvoidReuse(paths)
	info["pq_commitments_enabled"] = rpcWalletPqCommitmentsEnabled(paths)
	info["pq_carrier_enabled"] = rpcWalletPqCarrierEnabled(paths)
	if walletFormat == "hd" {
		info["hdchainid"] = int64(3)
		info["keypoolsize_hd_external"] = keypoolSize
		chgPool := 0
		if paths != nil && paths.WalletChangeKeypoolSize != nil {
			chgPool = paths.WalletChangeKeypoolSize()
		}
		info["keypoolsize_hd_internal"] = chgPool
		if paths != nil && paths.WalletHDSeedID != nil {
			if id := strings.TrimSpace(paths.WalletHDSeedID()); id != "" {
				info["hdseedid"] = id
			}
		}
		mergeWalletHDKeypoolCoreIndex(info, paths)
	}
	if len(rpcWalletWatchScripts(paths)) == 0 {
		info["watchonly_balance"] = 0.0
	} else {
		info["watchonly_balance"] = walletWatchonlyBalanceDOGE(chainName, paths, j, raw, pool)
	}
	if paths != nil && paths.WalletIsScanning != nil && paths.WalletIsScanning() {
		info["scanning"] = map[string]interface{}{
			"duration": 0,
		}
	}
	if paths != nil && paths.WalletMaxScannedBlockHeight != nil {
		idx := paths.WalletMaxScannedBlockHeight()
		info["wallet_index_height"] = idx
		tip := int64(-1)
		if paths.Utxo != nil {
			tip = paths.Utxo.TipHeight()
		} else if j != nil {
			if h, err := j.TipHeight(); err == nil {
				tip = h
			}
		}
		if tip >= 0 {
			info["chain_active_height"] = tip
			if idx >= 0 && idx < tip {
				info["needs_rescan"] = true
				info["rescan_from_height"] = idx + 1
			}
			if idx >= 0 {
				info["dogego_wallet_scan_index_ok"] = idx >= tip
			}
		}
	}
	if walletScanHasReceiveRows(paths) {
		info["dogego_wallet_history_fast_path"] = true
	} else {
		info["dogego_wallet_listtransactions_utxo_walk"] = true
		if paths != nil && paths.WalletIsScanning != nil && paths.WalletIsScanning() {
			info["dogego_wallet_listtransactions_scan_pending"] = true
		}
	}
	if paths != nil && paths.WalletPath != nil {
		if p := strings.TrimSpace(paths.WalletPath()); p != "" {
			if st, err := os.Stat(p); err == nil {
				info["keypoololdest"] = st.ModTime().Unix()
			}
		}
	}
	if paths != nil && len(paths.SignerCommand) > 0 {
		info["signer_cmd_configured"] = true
	}
	mergeWalletHistoryDeferReason(info, paths, j, chainName, raw)
	return info, 0, ""
}

func filepathBase(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func walletScriptSet(scripts [][]byte) map[string]struct{} {
	set := make(map[string]struct{}, len(scripts))
	for _, pk := range scripts {
		if len(pk) > 0 {
			set[hex.EncodeToString(pk)] = struct{}{}
		}
	}
	return set
}

// walletMempoolNetKoinuScripts sums mempool credits minus debits for the given scriptPubKeys.
func walletMempoolNetKoinuScripts(_ string, paths *DataPaths, pool *mempool.Pool, scripts [][]byte) int64 {
	if pool == nil || paths == nil || paths.Utxo == nil || len(scripts) == 0 {
		return 0
	}
	scriptSet := walletScriptSet(scripts)
	var bal int64
	entries, err := pool.SortedTransactions()
	if err != nil {
		return 0
	}
	for _, ent := range entries {
		tx, err := wire.DeserializeTx(ent.Raw)
		if err != nil {
			continue
		}
		var recv int64
		for _, o := range tx.Vout {
			if _, ok := scriptSet[hex.EncodeToString(o.PkScript)]; ok {
				recv += o.Value
			}
		}
		var spent int64
		for _, in := range tx.Vin {
			e, ok := paths.Utxo.LookupOutpoint(in.PrevHash, in.PrevIdx)
			if !ok {
				continue
			}
			if _, ok := scriptSet[hex.EncodeToString(e.PkScript)]; ok {
				spent += e.Value
			}
		}
		bal += recv - spent
	}
	return bal
}

// walletMempoolNetKoinu sums pending mempool credits minus debits for tracked wallet scripts.
func walletMempoolNetKoinu(chainName string, paths *DataPaths, pool *mempool.Pool) int64 {
	if rpcWalletAddress(paths) == "" && len(rpcWalletWatchScripts(paths)) == 0 {
		return 0
	}
	return walletMempoolNetKoinuScripts(chainName, paths, pool, rpcWalletTrackedScripts(paths))
}

// execGetUnconfirmedBalance returns the wallet mempool net balance (tracked scripts).
func execGetUnconfirmedBalance(chainName string, paths *DataPaths, pool *mempool.Pool, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 0 {
		return nil, -32602, "Wrong number of arguments"
	}
	if rpcWalletAddress(paths) == "" && len(rpcWalletWatchScripts(paths)) == 0 {
		return 0.0, 0, ""
	}
	return float64(walletMempoolNetKoinu(chainName, paths, pool)) / 1e8, 0, ""
}

// execGetReceivedByAddress sums wallet UTXO amounts (same cache as listunspent).
func execGetReceivedByAddress(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 3 {
		return nil, -32602, "Wrong number of arguments"
	}
	var addr string
	if err := json.Unmarshal(params[0], &addr); err != nil {
		return nil, -8, "getreceivedbyaddress: address must be a string"
	}
	addr = strings.TrimSpace(addr)
	vis, _, _ := ValidateAddressString(chainName, addr)
	if ok, _ := vis["isvalid"].(bool); !ok {
		return nil, -5, "Invalid Dogecoin address"
	}
	minConf := int64(1)
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[1], &n); err != nil {
			return nil, -8, "getreceivedbyaddress: minconf must be a number"
		}
		mi, err := n.Int64()
		if err != nil || mi < 0 {
			return nil, -8, "getreceivedbyaddress: minconf out of range"
		}
		minConf = mi
	}
	includeWatchonly := false
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var code int
		var msg string
		includeWatchonly, code, msg = parseRPCBoolOpt(params[2], false, "getreceivedbyaddress", "include_watchonly")
		if code != 0 {
			return nil, code, msg
		}
	}
	if !walletAddressIsTracked(paths, chainName, addr) {
		return 0.0, 0, ""
	}
	matches, code, msg := walletUtxoMatches(paths, j, raw, chainName, minConf, 0)
	if code != 0 {
		return nil, code, msg
	}
	var sum int64
	for _, m := range matches {
		if m.address != addr {
			continue
		}
		if !m.spendable && !includeWatchonly {
			continue
		}
		sum += m.row.Value
	}
	return float64(sum) / 1e8, 0, ""
}

type walletTxRow struct {
	txid          string
	category      string
	amountKoinu   int64
	confirmations int64
	blockHeight   int64 // -1 for mempool / unknown
	blockHash     string
	blockTime     int64 // header nTime when confirmed; else admission time
	bip125        string // "yes" | "no" | "unknown" (Core wallet tx list)
	abandoned     bool
	vout          uint32
	time          int64
	address       string
}

// walletCollectTransactions lists receive UTXOs and mempool wallet activity (newest first).
func walletCollectTransactions(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, minConf int64) ([]walletTxRow, int, string) {
	return walletCollectTransactionsImpl(chainName, paths, j, raw, pool, minConf, false)
}

// walletCollectTransactionsUI is the fast path for the web UI (no per-row header / tx-index lookups).
func walletCollectTransactionsUI(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, minConf int64) ([]walletTxRow, int, string) {
	return walletCollectTransactionsImpl(chainName, paths, j, raw, pool, minConf, true)
}

func walletCollectTransactionsImpl(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, minConf int64, light bool) ([]walletTxRow, int, string) {
	if rpcWalletAddress(paths) == "" && len(rpcWalletWatchScripts(paths)) == 0 {
		return nil, 0, ""
	}
	tracked := rpcWalletTrackedScripts(paths)
	if len(tracked) == 0 {
		return nil, 0, ""
	}
	skipUtxoReceives := light && walletScanHasReceiveRows(paths)
	var matches []walletUtxoMatch
	var code int
	var msg string
	if !skipUtxoReceives {
		matches, code, msg = walletUtxoMatches(paths, j, raw, chainName, minConf, 0)
		if code != 0 {
			return nil, code, msg
		}
	}
	now := time.Now().Unix()
	seen := make(map[string]struct{})
	var out []walletTxRow
	hashByHeight := make(map[int64]string)
	timeByHeight := make(map[int64]int64)

	appendRow := func(txid, category, addr string, amountKoinu int64, conf int64, vout uint32, blockHeight int64, abandoned bool) {
		if txid == "" {
			return
		}
		key := fmt.Sprintf("%s:%s:%s:%d", txid, category, addr, vout)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		rowTime := now
		bh := ""
		if blockHeight >= 0 {
			if light {
				rowTime = blockHeight
			} else {
				bh = hashByHeight[blockHeight]
				if bh == "" {
					bh = walletBlockHashAt(j, blockHeight)
					hashByHeight[blockHeight] = bh
				}
				t := timeByHeight[blockHeight]
				if t == 0 {
					t = walletHeaderTimeAt(j, blockHeight)
					if t > 0 {
						timeByHeight[blockHeight] = t
					}
				}
				if t > 0 {
					rowTime = t
				}
			}
		}
		bip125 := "no"
		if !abandoned && !light {
			bip125 = walletBIP125Replaceable(pool, txid, blockHeight)
		}
		out = append(out, walletTxRow{
			txid: txid, category: category, amountKoinu: amountKoinu,
			confirmations: conf, blockHeight: blockHeight, blockHash: bh, blockTime: rowTime,
			bip125: bip125, abandoned: abandoned,
			vout: vout, time: rowTime, address: addr,
		})
	}

	for _, m := range matches {
		appendRow(m.row.TxID, "receive", m.address, m.row.Value, m.confirmations, m.row.Vout, m.row.Height, false)
	}

	if paths != nil && paths.WalletListPrunedImports != nil {
		tip, _, _ := activeChainFromJournal(j, raw, paths)
		net, _ := networkFromRPCChainName(chainName)
		p, _ := chain.ParamsFor(net)
		for _, pi := range paths.WalletListPrunedImports() {
			conf := int64(1)
			if tip >= 0 && pi.BlockHeight >= 0 {
				conf = tip - pi.BlockHeight + 1
				if conf < 1 {
					conf = 1
				}
			}
			addr := chain.ScriptPubKeyAddress(pi.Script, p.PubkeyHashAddrID, p.ScriptHashAddrID)
			if addr == "" {
				addr = rpcWalletAddress(paths)
			}
			appendRow(pi.TxID, "receive", addr, pi.AmountKoinu, conf, pi.Vout, pi.BlockHeight, false)
		}
	}

	spendScripts := rpcWalletSpendScripts(paths)
	scriptAddr := make(map[string]string) // hex(script) -> address
	if net, err := networkFromRPCChainName(chainName); err == nil {
		if p, err := chain.ParamsFor(net); err == nil {
			for _, pk := range tracked {
				scriptAddr[hex.EncodeToString(pk)] = chain.ScriptPubKeyAddress(pk, p.PubkeyHashAddrID, p.ScriptHashAddrID)
			}
		}
	}
	if paths != nil && paths.WalletListScannedTx != nil {
		tip, _, _ := activeChainFromJournal(j, raw, paths)
		for _, st := range paths.WalletListScannedTx() {
			conf := int64(1)
			if tip >= 0 && st.BlockHeight >= 0 {
				conf = tip - st.BlockHeight + 1
				if conf < 1 {
					conf = 1
				}
			}
			addr := st.Address
			if addr == "" {
				addr = rpcWalletDefaultAddress(paths)
			}
			appendRow(st.TxID, st.Category, addr, st.AmountKoinu, conf, st.Vout, st.BlockHeight, false)
		}
	}
	if pool != nil && paths != nil && paths.Utxo != nil {
		entries, err := pool.SortedTransactions()
		if err == nil {
			for _, ent := range entries {
				tx, err := wire.DeserializeTx(ent.Raw)
				if err != nil {
					continue
				}
				recvByAddr := make(map[string]int64)
				var spent int64
				sendAddr := ""
				for _, o := range tx.Vout {
					addr, ok := scriptAddr[hex.EncodeToString(o.PkScript)]
					if !ok || addr == "" {
						continue
					}
					recvByAddr[addr] += o.Value
				}
				for _, in := range tx.Vin {
					e, ok := paths.Utxo.LookupOutpoint(in.PrevHash, in.PrevIdx)
					if !ok {
						continue
					}
					for _, sp := range spendScripts {
						if bytes.Equal(e.PkScript, sp) {
							spent += e.Value
							if sendAddr == "" {
								sendAddr = scriptAddr[hex.EncodeToString(sp)]
							}
							break
						}
					}
				}
				if spent > 0 {
					recv := int64(0)
					for _, v := range recvByAddr {
						recv += v
					}
					trackedSet := make(map[string][]byte, len(tracked))
					for _, pk := range tracked {
						if len(pk) > 0 {
							trackedSet[string(pk)] = pk
						}
					}
					netAmt := wallet.SendDisplayKoinu(spent, recvByAddr, sendAddr, tx.Vout, trackedSet)
					if sendAddr == "" {
						sendAddr = rpcWalletDefaultAddress(paths)
					}
					appendRow(ent.TxID, "send", sendAddr, -netAmt, 0, 0, -1, false)
				}
				for addr, recv := range recvByAddr {
					if recv > 0 {
						appendRow(ent.TxID, "receive", addr, recv, 0, 0, -1, false)
					}
				}
			}
		}
	}
	if paths != nil && paths.WalletListAbandoned != nil {
		for _, ab := range paths.WalletListAbandoned() {
			rowTime := ab.Time
			if rowTime <= 0 {
				rowTime = now
			}
			appendRow(ab.TxID, ab.Category, ab.Address, ab.AmountKoinu, 0, 0, -1, true)
			if rowTime > 0 {
				out[len(out)-1].time = rowTime
			}
		}
	}

	slices.SortFunc(out, func(a, b walletTxRow) int {
		if a.time != b.time {
			return int(b.time - a.time)
		}
		return strings.Compare(b.txid, a.txid)
	})
	out = walletCollapseUIHistoryRows(out)
	return out, 0, ""
}

// walletCollapseUIHistoryRows dedupes wallet UI history: one send per txid, one receive per txid+address.
func walletCollapseUIHistoryRows(rows []walletTxRow) []walletTxRow {
	if len(rows) <= 1 {
		return rows
	}
	sendByTx := make(map[string]int)
	recvByKey := make(map[string]int)
	out := make([]walletTxRow, 0, len(rows))
	for _, r := range rows {
		switch r.category {
		case "send":
			id := strings.ToLower(strings.TrimSpace(r.txid))
			if id == "" {
				out = append(out, r)
				continue
			}
			if i, ok := sendByTx[id]; ok {
				if abs64(r.amountKoinu) > abs64(out[i].amountKoinu) {
					out[i] = r
				}
				continue
			}
			sendByTx[id] = len(out)
			out = append(out, r)
		case "receive":
			key := strings.ToLower(strings.TrimSpace(r.txid)) + ":" + strings.TrimSpace(r.address)
			if i, ok := recvByKey[key]; ok {
				out[i].amountKoinu += r.amountKoinu
				if r.blockHeight > out[i].blockHeight {
					out[i].blockHeight = r.blockHeight
					out[i].confirmations = r.confirmations
					out[i].vout = r.vout
				}
				continue
			}
			recvByKey[key] = len(out)
			out = append(out, r)
		default:
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b walletTxRow) int {
		if a.time != b.time {
			return int(b.time - a.time)
		}
		return strings.Compare(b.txid, a.txid)
	})
	return out
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func walletTxRowFillHeaders(j HeaderJournal, r *walletTxRow) {
	if j == nil || r == nil || r.blockHeight < 0 {
		return
	}
	if r.blockHash == "" {
		r.blockHash = walletBlockHashCached(j, r.blockHeight)
	}
	if t := walletHeaderTimeCached(j, r.blockHeight); t > 0 {
		r.blockTime = t
		r.time = t
	}
}

func walletTxKindHeuristic(r walletTxRow, coinbaseMaturity int64) string {
	switch r.category {
	case "send":
		return "sent"
	case "receive":
		if r.vout == 0 {
			if r.confirmations > 0 && coinbaseMaturity > 0 && r.confirmations < coinbaseMaturity {
				return "mining_immature"
			}
			return "mining"
		}
		return "received"
	default:
		return r.category
	}
}

func walletTxRowToUIListEntry(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, ix *store.TxIndex, addr string, r walletTxRow, coinbaseMaturity int64) map[string]interface{} {
	if addr == "" {
		addr = rpcWalletAddress(paths)
	}
	lbl := rpcWalletGetLabel(paths, addr)
	kind := walletTxKindHeuristic(r, coinbaseMaturity)
	amtKoinu := r.amountKoinu
	if r.category == "send" {
		if sendAddr, displayAmt, ok := walletSendDisplayFromHex(chainName, paths, pool, ix, raw, j, r.txid, r.blockHeight); ok {
			amtKoinu = displayAmt
			if sendAddr != "" {
				addr = sendAddr
			}
		}
	}
	entry := map[string]interface{}{
		"account":            lbl,
		"label":              lbl,
		"address":            addr,
		"category":           r.category,
		"amount":             float64(amtKoinu) / 1e8,
		"confirmations":      r.confirmations,
		"txid":               r.txid,
		"time":               r.time,
		"timereceived":       r.time,
		"bip125-replaceable": r.bip125,
		"walletconflicts":    []interface{}{},
		"tx_kind":            kind,
	}
	if r.confirmations > 0 {
		entry["trusted"] = true
	}
	if r.category == "receive" {
		entry["vout"] = r.vout
	}
	if r.blockHeight >= 0 {
		entry["blockheight"] = r.blockHeight
		if r.blockHash != "" {
			entry["blockhash"] = r.blockHash
		}
		if r.blockTime > 0 {
			entry["blocktime"] = r.blockTime
		}
		if r.category == "receive" {
			entry["blockindex"] = r.vout
		}
	}
	if paths != nil && addr != "" {
		entry["iswatchonly"] = rpcWalletIsWatchAddress(paths, addr)
	}
	if r.abandoned {
		entry["abandoned"] = true
	}
	if r.category == "send" {
		if fee := walletSendFeeKoinu(chainName, paths, pool, ix, raw, j, r.txid, r.blockHeight); fee > 0 {
			entry["fee"] = float64(fee) / 1e8
		}
	}
	if r.category == "send" {
		if ek, pqTag := walletEnrichTxKindList(paths, pool, r); ek != "" {
			entry["tx_kind"] = ek
			if pqTag != "" {
				entry["pq_tag"] = pqTag
			}
		}
	} else if r.category == "receive" {
		if kind, pqTag := walletEnrichTxKind(chainName, paths, j, raw, pool, ix, r); kind != "" {
			entry["tx_kind"] = kind
			if pqTag != "" {
				entry["pq_tag"] = pqTag
			}
		}
	}
	return entry
}

func walletTxRowToListEntry(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, ix *store.TxIndex, addr string, r walletTxRow) map[string]interface{} {
	lbl := rpcWalletGetLabel(paths, addr)
	entry := map[string]interface{}{
		"account":            lbl,
		"label":              lbl,
		"address":            addr,
		"category":           r.category,
		"amount":             float64(r.amountKoinu) / 1e8,
		"confirmations":      r.confirmations,
		"txid":               r.txid,
		"time":               r.time,
		"timereceived":       r.time,
		"bip125-replaceable": r.bip125,
		"walletconflicts":    walletConflicts(chainName, paths, j, raw, pool, r.txid),
	}
	if r.confirmations > 0 {
		entry["trusted"] = true
	}
	if r.category == "receive" {
		entry["vout"] = r.vout
	}
	if r.blockHeight >= 0 {
		entry["blockheight"] = r.blockHeight
		if r.blockHash != "" {
			entry["blockhash"] = r.blockHash
		}
		if r.blockTime > 0 {
			entry["blocktime"] = r.blockTime
		}
		if r.category == "receive" {
			entry["blockindex"] = r.vout
		}
	}
	if paths != nil && addr != "" {
		entry["iswatchonly"] = rpcWalletIsWatchAddress(paths, addr)
	}
	if r.abandoned {
		entry["abandoned"] = true
	}
	if r.category == "send" {
		if fee := walletSendFeeKoinu(chainName, paths, pool, ix, raw, j, r.txid, r.blockHeight); fee > 0 {
			entry["fee"] = float64(fee) / 1e8
		}
		if kind, pqTag := walletEnrichTxKind(chainName, paths, j, raw, pool, ix, r); kind != "" {
			entry["tx_kind"] = kind
			if pqTag != "" {
				entry["pq_tag"] = pqTag
			}
		}
	} else if r.category == "receive" {
		entry["tx_kind"] = walletReceiveTxKind(chainName, paths, j, raw, ix, r)
	} else if kind, pqTag := walletEnrichTxKind(chainName, paths, j, raw, pool, ix, r); kind != "" {
		entry["tx_kind"] = kind
		if pqTag != "" {
			entry["pq_tag"] = pqTag
		}
	}
	return entry
}

// WalletListTransactions returns Core-shaped rows for the web UI (newest first).
func WalletListTransactions(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, ix *store.TxIndex) []interface{} {
	rows := walletUIRowsCached(chainName, paths, j, raw, pool)
	if len(rows) == 0 {
		return []interface{}{}
	}
	out := make([]interface{}, 0, len(rows))
	for _, r := range rows {
		walletTxRowFillHeaders(j, &r)
		addr := r.address
		if addr == "" {
			addr = rpcWalletAddress(paths)
		}
		out = append(out, walletTxRowToListEntry(chainName, paths, j, raw, pool, ix, addr, r))
	}
	return out
}

// walletRowAfterSinceBlock returns true when the row should appear in listsinceblock (Core: confirmed after block).
func walletRowAfterSinceBlock(r walletTxRow, sinceHeight int64, hasSince bool) bool {
	if !hasSince {
		return true
	}
	if r.blockHeight < 0 {
		return true // mempool activity
	}
	return r.blockHeight > sinceHeight
}

// execListSinceBlockWallet returns wallet receive UTXOs and mempool activity for the built-in wallet.
func execListSinceBlockWallet(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, ix *store.TxIndex, params []json.RawMessage) (interface{}, int, string) {
	sinceHeight, hasSince, minConf, includeWatchonly, code, msg := parseListSinceBlockParams(j, params)
	if code != 0 {
		return nil, code, msg
	}
	base, code, msg := execListSinceBlock(j, raw, paths, params)
	if code != 0 {
		return nil, code, msg
	}
	baseMap, _ := base.(map[string]interface{})
	last, _ := baseMap["lastblock"].(string)

	rows := walletUIRowsCached(chainName, paths, j, raw, pool)
	txs := make([]interface{}, 0, len(rows))
	for _, r := range rows {
		if r.confirmations < minConf {
			continue
		}
		if !walletRowAfterSinceBlock(r, sinceHeight, hasSince) {
			continue
		}
		if !walletRowMatchesFilter(paths, r, "", includeWatchonly) {
			continue
		}
		walletTxRowFillHeaders(j, &r)
		addr := walletRowAddress(paths, r)
		txs = append(txs, walletTxRowToListEntry(chainName, paths, j, raw, pool, ix, addr, r))
	}
	return map[string]interface{}{
		"transactions": txs,
		"lastblock":    last,
	}, 0, ""
}

// execListTransactionsWallet implements listtransactions for the built-in wallet.
func execListTransactionsWallet(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, ix *store.TxIndex, params []json.RawMessage) (interface{}, int, string) {
	if code, msg := execListTransactionsValidate(params); code != 0 {
		return nil, code, msg
	}
	account := ""
	if len(params) > 0 && strings.TrimSpace(string(params[0])) != "null" {
		_ = json.Unmarshal(params[0], &account)
	}
	count := int64(10)
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var n json.Number
		_ = json.Unmarshal(params[1], &n)
		count, _ = n.Int64()
	}
	skip := int64(0)
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var n json.Number
		_ = json.Unmarshal(params[2], &n)
		skip, _ = n.Int64()
	}
	includeWatchonly := false
	if len(params) > 3 && strings.TrimSpace(string(params[3])) != "null" {
		includeWatchonly, _, _ = parseRPCBoolOpt(params[3], false, "listtransactions", "include_watchonly")
	}
	rows := walletUIRowsCached(chainName, paths, j, raw, pool)
	filtered := rows[:0]
	for _, r := range rows {
		if walletRowMatchesFilter(paths, r, account, includeWatchonly) {
			filtered = append(filtered, r)
		}
	}
	rows = filtered
	if skip > int64(len(rows)) {
		skip = int64(len(rows))
	}
	end := skip + count
	if count <= 0 || end > int64(len(rows)) {
		end = int64(len(rows))
	}
	slice := rows[skip:end]
	out := make([]interface{}, 0, len(slice))
	for _, r := range slice {
		walletTxRowFillHeaders(j, &r)
		addr := walletRowAddress(paths, r)
		out = append(out, walletTxRowToListEntry(chainName, paths, j, raw, pool, ix, addr, r))
	}
	return out, 0, ""
}

func execListTransactionsValidate(params []json.RawMessage) (int, string) {
	if len(params) > 4 {
		return -32602, "Wrong number of arguments"
	}
	if len(params) > 0 && strings.TrimSpace(string(params[0])) != "null" {
		var account string
		if err := json.Unmarshal(params[0], &account); err != nil {
			return -8, "listtransactions: account must be a string"
		}
	}
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[1], &n); err != nil {
			return -8, "listtransactions: count must be a number"
		}
		ci, err := n.Int64()
		if err != nil || ci < 0 {
			return -8, "Negative count"
		}
	}
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[2], &n); err != nil {
			return -8, "listtransactions: skip must be a number"
		}
		si, err := n.Int64()
		if err != nil || si < 0 {
			return -8, "Negative from"
		}
	}
	if len(params) > 3 && strings.TrimSpace(string(params[3])) != "null" {
		if _, code, msg := parseRPCBoolOpt(params[3], false, "listtransactions", "include_watchonly"); code != 0 {
			return code, msg
		}
	}
	return 0, ""
}

// execGetTransactionWallet returns one wallet transaction by txid.
func execGetTransactionWallet(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, ix *store.TxIndex, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var hexStr string
	if err := json.Unmarshal(params[0], &hexStr); err != nil {
		return nil, -8, "gettransaction: txid must be a string"
	}
	txid := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexStr), "0x"))
	if len(txid) != 64 {
		return nil, -8, "gettransaction: invalid txid"
	}
	if _, err := chain.Hash256FromDisplayHex(txid); err != nil {
		return nil, -8, "gettransaction: invalid txid"
	}
	includeWatchonly := false
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var code int
		var msg string
		includeWatchonly, code, msg = parseRPCBoolOpt(params[1], false, "gettransaction", "include_watchonly")
		if code != 0 {
			return nil, code, msg
		}
	}
	rows := walletUIRowsCached(chainName, paths, j, raw, pool)
	matched := walletTxRowsForTxid(rows, txid, paths, includeWatchonly)
	if len(matched) == 0 {
		return nil, -5, "Invalid or non-wallet transaction id"
	}
	for i := range matched {
		walletTxRowFillHeaders(j, &matched[i])
	}
	primary := matched[0]
	addr := walletRowAddress(paths, primary)
	entry := walletTxRowToListEntry(chainName, paths, j, raw, pool, ix, addr, primary)
	entry["amount"] = float64(walletTxEntryAmountKoinu(matched)) / 1e8
	entry["details"] = walletTxDetailsFromRows(paths, matched)
	if walletTxInvolvesWatchonly(paths, matched) {
		entry["involvesWatchonly"] = true
	}
	if primary.blockTime > 0 {
		entry["blocktime"] = primary.blockTime
	}
	if primary.confirmations == 0 {
		entry["trusted"] = false
	}
	entry["hex"] = walletLookupTxHex(pool, paths, ix, raw, j, txid, primary.blockHeight)
	return entry, 0, ""
}


func buildWalletPrevTxs(tx *wire.Tx, paths *DataPaths) ([]json.RawMessage, error) {
	if paths == nil || paths.Utxo == nil || tx == nil {
		return nil, nil
	}
	var out []json.RawMessage
	for _, in := range tx.Vin {
		if isCoinbaseWireIn(&in) {
			continue
		}
		id := txidToRPC(in.PrevHash)
		e, ok := paths.Utxo.Lookup(id, in.PrevIdx)
		if !ok {
			continue
		}
		ent := map[string]interface{}{
			"txid":         id,
			"vout":         in.PrevIdx,
			"scriptPubKey": hex.EncodeToString(e.PkScript),
		}
		if paths.WalletWatchRedeemScript != nil {
			if redeem := paths.WalletWatchRedeemScript(e.PkScript); len(redeem) > 0 {
				ent["redeemScript"] = hex.EncodeToString(redeem)
			}
		}
		b, err := json.Marshal(ent)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}
