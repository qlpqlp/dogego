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

type balanceBucket struct {
	trusted          float64
	untrustedPending float64
	immature         float64
}

// execGetBalances returns Core-shaped mine / watchonly balance buckets.
func execGetBalances(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, ix *store.TxIndex, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 0 {
		return nil, -32602, "Wrong number of arguments"
	}
	empty := func() map[string]interface{} {
		z := map[string]interface{}{
			"trusted":            0.0,
			"untrusted_pending":  0.0,
			"immature":           0.0,
		}
		return map[string]interface{}{
			"mine":       z,
			"watchonly":  z,
		}
	}
	if rpcWalletAddress(paths) == "" && len(rpcWalletWatchScripts(paths)) == 0 {
		return empty(), 0, ""
	}
	coinbaseMaturity := walletCoinbaseMaturity(chainName, j, raw, paths)
	matches, code, msg := walletUtxoMatches(paths, j, raw, chainName, 0, 0)
	if code != 0 {
		return nil, code, msg
	}
	var mine, watch balanceBucket
	for _, m := range matches {
		conf := m.confirmations
		doge := float64(m.row.Value) / 1e8
		b := &watch
		if m.spendable {
			b = &mine
		}
		if conf >= coinbaseMaturity {
			b.trusted += doge
		} else if conf > 0 && walletUtxoImmatureCoinbase(m.row, ix, raw) {
			b.immature += doge
		} else if conf > 0 {
			b.trusted += doge
		}
	}
	if pool != nil {
		mineScripts := rpcWalletSpendScripts(paths)
		watchScripts := rpcWalletWatchScripts(paths)
		mine.untrustedPending = float64(walletMempoolNetKoinuScripts(chainName, paths, pool, mineScripts)) / 1e8
		watch.untrustedPending = float64(walletMempoolNetKoinuScripts(chainName, paths, pool, watchScripts)) / 1e8
	}
	toObj := func(b balanceBucket) map[string]interface{} {
		return map[string]interface{}{
			"trusted":           b.trusted,
			"untrusted_pending": b.untrustedPending,
			"immature":          b.immature,
		}
	}
	return map[string]interface{}{
		"mine":      toObj(mine),
		"watchonly": toObj(watch),
	}, 0, ""
}

func walletWatchonlyBalanceDOGE(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool) float64 {
	bal, code, _ := execGetBalances(chainName, paths, j, raw, pool, nil, nil)
	if code != 0 {
		return 0
	}
	m, ok := bal.(map[string]interface{})
	if !ok {
		return 0
	}
	wo, ok := m["watchonly"].(map[string]interface{})
	if !ok {
		return 0
	}
	var sum float64
	for _, k := range []string{"trusted", "untrusted_pending", "immature"} {
		if v, ok := wo[k].(float64); ok {
			sum += v
		}
	}
	return sum
}
