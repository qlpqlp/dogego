// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"

	"dogego/mempool"
	"dogego/store"
)

// execListStuckTransactionsWallet returns wallet send txs with 0 confirmations not in the mempool.
func execListStuckTransactionsWallet(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, ix *store.TxIndex, params []json.RawMessage) (interface{}, int, string) {
	verbose, includeWatchonly, code, msg := execListStuckTransactionsValidate(params)
	if code != 0 {
		return nil, code, msg
	}
	if rpcWalletAddress(paths) == "" {
		return []interface{}{}, 0, ""
	}
	rows := walletUIRowsCached(chainName, paths, j, raw, pool)
	inMempool := make(map[string]struct{})
	if pool != nil {
		ids, _ := pool.RawMemPoolTxIDs()
		for _, id := range ids {
			inMempool[strings.ToLower(id)] = struct{}{}
		}
	}
	var out []interface{}
	for _, r := range rows {
		if r.category != "send" || r.confirmations > 0 {
			continue
		}
		if !walletRowMatchesFilter(paths, r, "", includeWatchonly) {
			continue
		}
		if _, ok := inMempool[strings.ToLower(r.txid)]; ok {
			continue
		}
		addr := walletRowAddress(paths, r)
		walletTxRowFillHeaders(j, &r)
		entry := walletTxRowToListEntry(chainName, paths, j, raw, pool, ix, addr, r)
		if verbose {
			entry["hex"] = walletLookupTxHex(pool, paths, ix, raw, j, r.txid, r.blockHeight)
			feeKoinu := walletSendFeeKoinu(chainName, paths, pool, ix, raw, j, r.txid, r.blockHeight)
			entry["fee"] = float64(feeKoinu) / 1e8
		}
		out = append(out, entry)
	}
	if out == nil {
		out = []interface{}{}
	}
	return out, 0, ""
}

func execListStuckTransactionsValidate(params []json.RawMessage) (verbose, includeWatchonly bool, code int, msg string) {
	if len(params) > 2 {
		return false, false, -32602, "Wrong number of arguments"
	}
	if len(params) > 0 && strings.TrimSpace(string(params[0])) != "null" {
		var c int
		var m string
		verbose, c, m = parseRPCBoolOpt(params[0], false, "liststucktransactions", "verbose")
		if c != 0 {
			return false, false, c, m
		}
	}
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var c int
		var m string
		includeWatchonly, c, m = parseRPCBoolOpt(params[1], false, "liststucktransactions", "include_watchonly")
		if c != 0 {
			return false, false, c, m
		}
	}
	return verbose, includeWatchonly, 0, ""
}
