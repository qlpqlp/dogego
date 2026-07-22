// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"

	"dogego/chain"
	"dogego/store"
	"dogego/wallet"
)

type lockUnspentOut struct {
	Txid string `json:"txid"`
	Vout int    `json:"vout"`
}

func rpcWalletIsLockedOutpoint(paths *DataPaths, txid string, vout uint32) bool {
	if paths == nil || paths.WalletIsLockedOutpoint == nil {
		return false
	}
	return paths.WalletIsLockedOutpoint(txid, vout)
}

// execListLockUnspentWallet returns locked outpoints for the built-in wallet.
func execListLockUnspentWallet(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 0 {
		return nil, -32602, "Wrong number of arguments"
	}
	if paths == nil || paths.WalletListLocked == nil || rpcWalletAddress(paths) == "" {
		return []interface{}{}, 0, ""
	}
	locked := paths.WalletListLocked()
	out := make([]interface{}, 0, len(locked))
	for _, o := range locked {
		out = append(out, map[string]interface{}{
			"txid": o.TxID,
			"vout": o.Vout,
		})
	}
	return out, 0, ""
}

// execLockUnspentWallet locks or unlocks outpoints in wallet.json.
func execLockUnspentWallet(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	unlock := false
	if _, code, msg := parseRPCBoolOpt(params[0], false, "lockunspent", "unlock"); code != 0 {
		return nil, code, msg
	}
	if len(params) == 1 {
		if paths == nil || paths.WalletSetLocked == nil || rpcWalletAddress(paths) == "" {
			return true, 0, ""
		}
		if err := paths.WalletSetLocked(unlock, nil); err != nil {
			return nil, -1, "lockunspent: " + err.Error()
		}
		return true, 0, ""
	}
	if strings.TrimSpace(string(params[1])) == "null" {
		return nil, -8, "lockunspent: transactions must be a JSON array"
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(params[1], &arr); err != nil {
		return nil, -8, "lockunspent: transactions must be a JSON array"
	}
	var outs []wallet.LockedOutpoint
	for _, elem := range arr {
		var o lockUnspentOut
		if err := json.Unmarshal(elem, &o); err != nil {
			return nil, -8, "lockunspent: Invalid parameter, expected object"
		}
		txid := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(o.Txid), "0x"))
		if len(txid) != 64 {
			return nil, -8, "lockunspent: Invalid parameter, expected hex txid"
		}
		if _, err := chain.Hash256FromDisplayHex(txid); err != nil {
			return nil, -8, "lockunspent: Invalid parameter, expected hex txid"
		}
		if o.Vout < 0 {
			return nil, -8, "lockunspent: Invalid parameter, vout must be positive"
		}
		outs = append(outs, wallet.LockedOutpoint{TxID: txid, Vout: uint32(o.Vout)})
	}
	if paths == nil || paths.WalletSetLocked == nil || rpcWalletAddress(paths) == "" {
		return true, 0, ""
	}
	if err := paths.WalletSetLocked(unlock, outs); err != nil {
		return nil, -1, "lockunspent: " + err.Error()
	}
	return true, 0, ""
}

// utxoRowFundable reports whether a UTXO may be used to fund a transaction.
// When includeWatching is false (Core default), watch-only scripts are excluded.
func utxoRowFundable(chainName string, paths *DataPaths, pkScript []byte, p chain.Params, includeWatching bool) bool {
	return utxoRowFundableSets(chainName, paths, buildFundScriptSets(paths), pkScript, p, includeWatching)
}

type fundScriptSets struct {
	spend    map[string]struct{}
	watch    map[string]struct{}
	hasSpend bool
	hasWatch bool
}

func buildFundScriptSets(paths *DataPaths) fundScriptSets {
	out := fundScriptSets{
		spend: make(map[string]struct{}),
		watch: make(map[string]struct{}),
	}
	for _, spend := range rpcWalletSpendScripts(paths) {
		if len(spend) > 0 {
			out.spend[string(spend)] = struct{}{}
			out.hasSpend = true
		}
	}
	for _, w := range rpcWalletWatchScripts(paths) {
		if len(w) > 0 {
			out.watch[string(w)] = struct{}{}
			out.hasWatch = true
		}
	}
	return out
}

func utxoRowFundableSets(chainName string, paths *DataPaths, sets fundScriptSets, pkScript []byte, p chain.Params, includeWatching bool) bool {
	key := string(pkScript)
	if _, ok := sets.spend[key]; ok {
		return true
	}
	if includeWatching {
		if _, ok := sets.watch[key]; ok {
			return true
		}
	} else if walletWatchScriptFundable(chainName, paths, pkScript) {
		return true
	}
	if sets.hasSpend || sets.hasWatch {
		return false
	}
	return isFundableP2PKH(pkScript, p.PubkeyHashAddrID)
}

func utxoOutpointLocked(paths *DataPaths, row store.UtxoDumpRow) bool {
	return rpcWalletIsLockedOutpoint(paths, row.TxID, row.Vout)
}
