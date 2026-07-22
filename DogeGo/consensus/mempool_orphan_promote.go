// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"dogego/mempool"
	"dogego/wire"
)

// PromoteOrphansForBlock tries to admit orphan descendants of each non-coinbase tx in a stored block.
func PromoteOrphansForBlock(pb *wire.ParsedBlock, pool *mempool.Pool, orphans OrphanStore, adm MempoolAdmission) {
	if pb == nil || pool == nil || orphans == nil {
		return
	}
	for i, tx := range pb.Txs {
		if i == 0 || IsCoinbaseTx(tx) {
			continue
		}
		promoteOrphanChildren(pool, orphans, adm, txidDisplayFromLE(tx.TxHash()))
	}
}

// PromoteOrphansForBlockRaw is PromoteOrphansForBlock on serialized block bytes (no ParseBlock).
func PromoteOrphansForBlockRaw(blockRaw []byte, pool *mempool.Pool, orphans OrphanStore, adm MempoolAdmission) {
	if len(blockRaw) < 80 || pool == nil || orphans == nil {
		return
	}
	_ = wire.ForEachBlockTx(blockRaw, func(i uint32, tx *wire.Tx) error {
		if i == 0 || IsCoinbaseTx(tx) {
			return nil
		}
		promoteOrphanChildren(pool, orphans, adm, txidDisplayFromLE(tx.TxHash()))
		return nil
	})
}
