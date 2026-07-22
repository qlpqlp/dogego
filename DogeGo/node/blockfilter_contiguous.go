// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"dogego/pow"
	"dogego/rpc"
	"dogego/store"
)

// indexBlockFiltersRange builds BIP158 basic filters for journal heights [from, to] inclusive.
func indexBlockFiltersRange(from, to int64, j *store.HeaderJournal, raw *store.RawBlockStore, filters *store.BlockFilterIndex, txIx *store.TxIndex) {
	if j == nil || raw == nil || filters == nil || txIx == nil || to < 0 {
		return
	}
	if from < 0 {
		from = 0
	}
	for h := from; h <= to; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return
		}
		id := pow.BlockHashLE(h80)
		blockRaw, err := raw.Get(id)
		if err != nil {
			continue
		}
		_ = rpc.IndexBasicBlockFilter(filters, id, blockRaw, j, raw, txIx)
	}
}

// blockFilterIndexOnPut indexes a filter only when the block is on the contiguous raw chain.
func blockFilterIndexOnPut(bs *BlockStoreCtx, j *store.HeaderJournal, filters *store.BlockFilterIndex, raw *store.RawBlockStore, txIx *store.TxIndex, hashLE [32]byte, blockRaw []byte) error {
	if filters == nil || txIx == nil {
		return nil
	}
	// During deep body IBD, skip filter writes on the download path (backfill via chainActive / reindex).
	if ShouldDeferTxIndexOnPut(bs) {
		return nil
	}
	if j == nil || len(blockRaw) < 80 {
		return rpc.IndexBasicBlockFilter(filters, hashLE, blockRaw, j, raw, txIx)
	}
	display := pow.BlockHashHex(blockRaw[:80])
	h, err := j.HeightByDisplayHash(display)
	if err != nil {
		return nil
	}
	if bs != nil {
		cont := bs.ContiguousRawHeight()
		if cont >= 0 && h > cont {
			return nil
		}
	}
	return rpc.IndexBasicBlockFilter(filters, hashLE, blockRaw, j, raw, txIx)
}
