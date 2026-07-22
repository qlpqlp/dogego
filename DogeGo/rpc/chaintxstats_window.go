// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

// countWindowTxsFromRawBlocks sums transaction counts for heights [start, tip] from stored raw blocks.
func countWindowTxsFromRawBlocks(j HeaderJournal, raw *store.RawBlockStore, start, tip int64) (int64, bool) {
	if j == nil || raw == nil || start < 0 || tip < start {
		return 0, false
	}
	var total int64
	var got int64
	for h := start; h <= tip; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			continue
		}
		id := pow.BlockHashLE(h80)
		payload, err := raw.Get(id)
		if err != nil {
			continue
		}
		nTx, err := wire.BlockTxCount(payload)
		if err != nil {
			continue
		}
		total += int64(nTx)
		got++
	}
	if got == 0 {
		return 0, false
	}
	return total, true
}
