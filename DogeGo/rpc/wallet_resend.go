// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"

	"dogego/mempool"
	"dogego/store"
)

// execResendWalletTransactionsWallet re-broadcasts unconfirmed wallet sends from the mempool.
func execResendWalletTransactionsWallet(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, params []json.RawMessage, relayTx func([]byte) error) (interface{}, int, string) {
	if len(params) != 0 {
		return nil, -32602, "Wrong number of arguments"
	}
	if pool == nil || relayTx == nil || rpcWalletAddress(paths) == "" {
		return []interface{}{}, 0, ""
	}
	rows := walletUIRowsCached(chainName, paths, j, raw, pool)
	sent := make([]interface{}, 0)
	seen := make(map[string]struct{})
	for _, r := range rows {
		if r.category != "send" || r.confirmations != 0 {
			continue
		}
		if _, ok := seen[r.txid]; ok {
			continue
		}
		raw, err := pool.GetRawByTxID(r.txid)
		if err != nil {
			continue
		}
		if err := relayTx(raw); err != nil {
			continue
		}
		seen[r.txid] = struct{}{}
		sent = append(sent, r.txid)
	}
	return sent, 0, ""
}
