// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"strconv"
	"time"

	"dogego/applog"
	"dogego/store"
)

// startBlockFilterCatchUpWorker indexes deferred BIP158 filters during body IBD (bounded batches).
func startBlockFilterCatchUpWorker(ctx context.Context, bs *BlockStoreCtx, j *store.HeaderJournal, raw *store.RawBlockStore, filters *store.BlockFilterIndex, txIx *store.TxIndex, lastFilter *int64) {
	if bs == nil || j == nil || raw == nil || filters == nil || txIx == nil || lastFilter == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !BodiesBehindHeaders(bs) {
					continue
				}
				if utxo := bs.Utxo; utxo != nil && ConnectCatchUpLag(bs, utxo) > 512 {
					continue
				}
				cont := bs.ContiguousRawHeight()
				if cont < 0 {
					continue
				}
				last := *lastFilter
				if last < 0 {
					continue
				}
				from := last + 1
				if cont < from {
					continue
				}
				to := from + blockFilterIndexMaxPerContiguousAdvance - 1
				if to > cont {
					to = cont
				}
				indexBlockFiltersRange(from, to, j, raw, filters, txIx)
				*lastFilter = to
				SetFilterIndexThrough(to)
				if to-from+1 >= blockFilterIndexMaxPerContiguousAdvance || to%256 == 0 {
					applog.Line("index", "filter catch-up: indexed heights "+strconv.FormatInt(from, 10)+
						"…"+strconv.FormatInt(to, 10)+" (contiguous "+strconv.FormatInt(cont, 10)+")")
				}
			}
		}
	}()
}
