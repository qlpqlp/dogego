// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"time"
)

const replayRampWorkerInterval = 20 * time.Second

// startReplayRampWorker periodically advances contiguous coverage from on-disk bodies during
// UTXO-snapshot-ahead replay (parallel fetch may store bodies far ahead of cached contiguous).
func startReplayRampWorker(ctx context.Context, bs *BlockStoreCtx) {
	if bs == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(replayRampWorkerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !bs.utxoAheadOfStoredBodies() {
					continue
				}
				passes := 12
				if bs.Utxo != nil && bs.Utxo.TipHeight() >= 0 {
					if remain := bs.Utxo.TipHeight() - bs.ContiguousRawHeight(); remain > 0 && remain <= 128 {
						passes = 32
					}
				}
				rampReplayContiguousFromDiskBounded(bs, passes)
			}
		}
	}()
}
