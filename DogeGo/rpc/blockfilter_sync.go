// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"dogego/pow"
	"dogego/store"
)

// BlockFilterSyncedHeight returns the highest chainActive height with a stored basic filter, or -1.
func BlockFilterSyncedHeight(j HeaderJournal, chainHeight int64, f *store.BlockFilterIndex) int64 {
	if j == nil || f == nil || chainHeight < 0 {
		return -1
	}
	for h := chainHeight; h >= 0; h-- {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			continue
		}
		if f.Has(pow.BlockHashLE(h80)) {
			return h
		}
	}
	return -1
}
