// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"

	"dogego/applog"
	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

func reloadPersistedMempool(
	pool *mempool.Pool,
	chainDataDir string,
	utxo *store.UtxoCache,
	txIx *store.TxIndex,
	rb *store.RawBlockStore,
	j consensus.HeaderChain,
	net chain.Network,
	standard consensus.StandardPolicy,
	limits consensus.MempoolRelayLimits,
	fullRBF bool,
	feeHistory *consensus.FeeHistory,
) {
	if pool == nil || chainDataDir == "" {
		return
	}
	adm := consensus.NewMempoolAdmissionWithUtxo(pool, pool, utxo, txIx, rb, j, net)
	adm.Standard = standard
	adm.FullRBF = fullRBF
	limits.Apply(&adm)
	adm.MinRelayFeePerKB = consensus.EffectiveMinRelayFeePerKB(0, pool.MinRelayFeePerKB())
	path := mempool.PersistPath(chainDataDir)
	snap, err := mempool.LoadPersistedSnapshot(path)
	if err != nil {
		applog.Line("mempool", "persist load: "+err.Error())
		return
	}
	loaded, skipped := 0, 0
	for _, rawTx := range snap.Transactions {
		tx, derr := wire.DeserializeTx(rawTx)
		if derr != nil {
			skipped++
			continue
		}
		if err := consensus.AcceptMempoolTxWithOrphans(rawTx, tx, pool, nil, adm, "persist"); err != nil {
			skipped++
			continue
		}
		loaded++
	}
	if loaded > 0 || skipped > 0 {
		applog.Line("mempool", fmt.Sprintf("restored %d transaction(s) from dogego_mempool.json (%d skipped)", loaded, skipped))
	}
	pool.RestoreFeeDeltas(snap.FeeDeltas)
	if feeHistory != nil && loaded > 0 {
		tip, err := j.TipHeight()
		if err == nil && tip >= 0 {
			if n := feeHistory.RehydrateFromPool(pool, adm.View, tip); n > 0 {
				applog.Line("mempool", fmt.Sprintf("rehydrated %d fee-estimator track(s) from mempool", n))
			}
		}
	}
}
