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

// execAbandonTransactionWallet removes a wallet mempool transaction from the pool.
func execAbandonTransactionWallet(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	txid, code, msg := parseOneTxidParam(params, "abandontransaction")
	if code != 0 {
		return nil, code, msg
	}
	if pool == nil {
		return nil, -1, "abandontransaction: mempool not available"
	}
	if rpcWalletAddress(paths) == "" {
		return nil, -1, "abandontransaction: wallet is not implemented in DogeGo"
	}
	if !pool.ContainsTxID(txid) {
		return nil, -5, "Invalid or non-wallet transaction id"
	}
	if !walletKnowsTxid(chainName, paths, j, raw, pool, txid) {
		return nil, -5, "Invalid or non-wallet transaction id"
	}
	cat, addr, amt, ok := walletMempoolTxSnapshot(chainName, paths, pool, txid)
	if !ok {
		return nil, -5, "Invalid or non-wallet transaction id"
	}
	if paths.WalletAbandonTx != nil {
		if err := paths.WalletAbandonTx(txid, cat, addr, amt); err != nil {
			return nil, -4, "abandontransaction: "+err.Error()
		}
	}
	if paths.WalletRemoveReplacementsForTx != nil {
		_ = paths.WalletRemoveReplacementsForTx(txid)
	}
	if !pool.RemoveByTxID(txid) {
		return nil, -5, "Invalid or non-wallet transaction id"
	}
	return nil, 0, ""
}

// execAbandonTransaction is used when no built-in wallet address is configured (Core requires wallet).
func execAbandonTransaction(pool *mempool.Pool, params []json.RawMessage) (interface{}, int, string) {
	_ = pool
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	return nil, -1, "abandontransaction: wallet is not implemented in DogeGo"
}
