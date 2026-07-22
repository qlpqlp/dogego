// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"fmt"
	"os"

	"dogego/pow"
)

// PruneChainDataAboveHeight removes raw blocks and tx index entries for heights (keepThrough+1)..tip
// before the header journal is truncated. Call while the journal still reflects the old tip.
func PruneChainDataAboveHeight(j *HeaderJournal, raw *RawBlockStore, txIx *TxIndex, keepThrough int64) (blocksRemoved int, err error) {
	if j == nil {
		return 0, fmt.Errorf("prune: nil journal")
	}
	j.ReconcileCountCacheFromDisk()
	tip, err := j.TipHeight()
	if err != nil {
		return 0, err
	}
	if keepThrough >= tip {
		return 0, nil
	}
	for h := keepThrough + 1; h <= tip; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return blocksRemoved, fmt.Errorf("prune read header %d: %w", h, err)
		}
		id := pow.BlockHashLE(h80)
		if raw != nil && raw.Has(id) {
			if err := raw.Remove(id); err != nil && !os.IsNotExist(err) {
				return blocksRemoved, fmt.Errorf("prune raw height %d: %w", h, err)
			}
			blocksRemoved++
		}
		if txIx != nil {
			if err := txIx.RemoveBlock(id); err != nil {
				return blocksRemoved, fmt.Errorf("prune tx index height %d: %w", h, err)
			}
		}
	}
	return blocksRemoved, nil
}
