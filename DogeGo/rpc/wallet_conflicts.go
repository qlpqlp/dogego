// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"sort"
	"strings"

	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

// walletConflicts returns Core-shaped walletconflicts for listtransactions/gettransaction.
func walletConflicts(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, txid string) []interface{} {
	seen := make(map[string]struct{})
	var ids []string
	for _, id := range walletStoredConflicts(paths, txid) {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, id := range walletMempoolConflictTxids(chainName, paths, j, raw, pool, txid) {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]interface{}, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

func walletStoredConflicts(paths *DataPaths, txid string) []string {
	if paths == nil || paths.WalletConflictsForTx == nil {
		return nil
	}
	return paths.WalletConflictsForTx(txid)
}

// walletMempoolConflictTxids returns other wallet txs in mempool that double-spend inputs with txid.
func walletMempoolConflictTxids(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, txid string) []string {
	if pool == nil || txid == "" {
		return nil
	}
	rawTx, err := pool.GetRawByTxID(txid)
	if err != nil || len(rawTx) == 0 {
		return nil
	}
	tx, err := wire.DeserializeTx(rawTx)
	if err != nil {
		return nil
	}
	spent := walletTxInputKeys(tx)
	if len(spent) == 0 {
		return nil
	}
	entries, err := pool.SortedTransactions()
	if err != nil {
		return nil
	}
	var out []string
	for _, ent := range entries {
		other := strings.ToLower(strings.TrimSpace(ent.TxID))
		if other == "" || strings.EqualFold(other, txid) {
			continue
		}
		if !walletKnowsTxid(chainName, paths, j, raw, pool, other) {
			continue
		}
		otherRaw, err := pool.GetRawByTxID(other)
		if err != nil {
			continue
		}
		otx, err := wire.DeserializeTx(otherRaw)
		if err != nil {
			continue
		}
		if walletTxSharesInput(otx, spent) {
			out = append(out, other)
		}
	}
	return out
}

func walletTxInputKeys(tx *wire.Tx) map[string]struct{} {
	if tx == nil {
		return nil
	}
	out := make(map[string]struct{})
	for i := range tx.Vin {
		in := &tx.Vin[i]
		if isCoinbaseWireIn(in) {
			continue
		}
		out[prevMapKey(in.PrevHash, in.PrevIdx)] = struct{}{}
	}
	return out
}

func walletTxSharesInput(tx *wire.Tx, spent map[string]struct{}) bool {
	if tx == nil || len(spent) == 0 {
		return false
	}
	for i := range tx.Vin {
		in := &tx.Vin[i]
		if isCoinbaseWireIn(in) {
			continue
		}
		if _, ok := spent[prevMapKey(in.PrevHash, in.PrevIdx)]; ok {
			return true
		}
	}
	return false
}
