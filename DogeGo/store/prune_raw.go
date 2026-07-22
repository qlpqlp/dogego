// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"fmt"

	"dogego/pow"
)

// PruneRawBlocksBelowHeight removes stored raw blocks (and tx index entries) for heights [0, belowHeight).
// Headers in headers.bin are not modified. Returns the height of the last pruned block and count removed.
func PruneRawBlocksBelowHeight(j *HeaderJournal, raw *RawBlockStore, txIx *TxIndex, belowHeight int64) (lastPruned int64, removed int, err error) {
	if j == nil {
		return 0, 0, fmt.Errorf("prune: nil journal")
	}
	if belowHeight <= 0 {
		return 0, 0, nil
	}
	tip, err := j.TipHeight()
	if err != nil {
		return 0, 0, err
	}
	if belowHeight > tip+1 {
		belowHeight = tip + 1
	}
	lastPruned = belowHeight - 1
	for h := int64(0); h < belowHeight; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return lastPruned, removed, fmt.Errorf("prune read header %d: %w", h, err)
		}
		id := pow.BlockHashLE(h80)
		if raw != nil && raw.Has(id) {
			if err := raw.Remove(id); err != nil {
				return lastPruned, removed, fmt.Errorf("prune raw height %d: %w", h, err)
			}
			removed++
		}
		if txIx != nil {
			if err := txIx.RemoveBlock(id); err != nil {
				return lastPruned, removed, fmt.Errorf("prune tx index height %d: %w", h, err)
			}
		}
	}
	return lastPruned, removed, nil
}
