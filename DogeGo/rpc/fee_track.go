// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

// TrackMempoolTxFee records mempool admission feerate for fee estimation when wired.
func TrackMempoolTxFee(paths *DataPaths, tx *wire.Tx, raw []byte, pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore, j HeaderJournal, net chain.Network) {
	if paths == nil || paths.FeeHistory == nil || tx == nil {
		return
	}
	view := mempoolAdmissionView(pool, txIndex, blocks)
	if view == nil {
		return
	}
	height := int64(-1)
	if paths.HeaderTipHeight != nil {
		height = paths.HeaderTipHeight()
	}
	if height < 0 {
		return
	}
	consensus.TrackMempoolTxFee(paths.FeeHistory, tx, raw, view, height)
}
