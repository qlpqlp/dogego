// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"dogego/store"
)

// blockFilterIndexMaxPerContiguousAdvance caps filter work per catch-up tick (and formerly IBD drip).
const blockFilterIndexMaxPerContiguousAdvance = 32

// onContiguousAdvanceIndexFilters indexes BIP158 filters after contiguous raw advance.
// During body IBD, skip drip indexing entirely (filter catch-up worker backfills) so getdata owns IO.
func onContiguousAdvanceIndexFilters(bs *BlockStoreCtx, lastFilter *int64, cont int64, j *store.HeaderJournal, raw *store.RawBlockStore, filters *store.BlockFilterIndex, txIx *store.TxIndex) {
	if lastFilter == nil || j == nil || raw == nil || cont < 0 {
		return
	}
	if BodiesBehindHeaders(bs) {
		// Seed watermark once (skip huge [0..cont] restart backfill). Do not drip-index or
		// advance the watermark here â€” startBlockFilterCatchUpWorker indexes when connect lag is low.
		if *lastFilter < 0 {
			*lastFilter = cont
			SetFilterIndexThrough(cont)
		}
		return
	}
	if filters == nil || txIx == nil {
		return
	}
	from := *lastFilter + 1
	if *lastFilter < 0 {
		from = 0
	}
	if cont >= from {
		indexBlockFiltersRange(from, cont, j, raw, filters, txIx)
	}
	*lastFilter = cont
	SetFilterIndexThrough(cont)
}
