// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

// blockFetchWantHeight returns the next block height forward IBD is trying to fetch.
func blockFetchWantHeight(bs *BlockStoreCtx) int64 {
	if bs == nil {
		return -1
	}
	h := bs.ContiguousRawHeight() + 1
	if h < 0 {
		return 0
	}
	return h
}
