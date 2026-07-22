// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"strings"

	"dogego/mempool"
	"dogego/store"
)

// walletRowAddress resolves the display address for a wallet transaction row.
func walletRowAddress(paths *DataPaths, r walletTxRow) string {
	addr := strings.TrimSpace(r.address)
	if addr != "" {
		return addr
	}
	return rpcWalletAddress(paths)
}

// walletRowMatchesFilter applies listtransactions account (label) and include_watchonly filters.
func walletRowMatchesFilter(paths *DataPaths, r walletTxRow, account string, includeWatchonly bool) bool {
	addr := walletRowAddress(paths, r)
	if !includeWatchonly && rpcWalletIsWatchAddress(paths, addr) {
		return false
	}
	account = strings.TrimSpace(account)
	if account == "" || account == "*" {
		return true
	}
	return rpcWalletGetLabel(paths, addr) == account
}

// walletKnowsTxid reports whether txid appears in wallet UTXO or mempool activity.
func walletKnowsTxid(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, txid string) bool {
	rows := walletUIRowsCached(chainName, paths, j, raw, pool)
	if len(rows) == 0 {
		return false
	}
	for _, r := range rows {
		if strings.EqualFold(r.txid, txid) {
			return true
		}
	}
	return false
}
