// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

// ShouldDeferInboundHeaders is true when extending the header chain would outrun block
// download (Core prioritizes block bodies during forward IBD).
func ShouldDeferInboundHeaders(bs *BlockStoreCtx) bool {
	if bs == nil || bs.Journal == nil {
		return false
	}
	if !BodiesBehindHeaders(bs) {
		return false
	}
	tip, err := bs.Journal.TipHeight()
	if err != nil || tip < 1 {
		return false
	}
	return ShouldDeferTipBackfill(tip, bs.ContiguousRawHeight())
}

// ShouldAnnounceConnectedBlocks reports whether to relay inv for newly connected blocks.
func ShouldAnnounceConnectedBlocks(bs *BlockStoreCtx) bool {
	if bs == nil || bs.Journal == nil {
		return false
	}
	tip, err := bs.Journal.TipHeight()
	if err != nil || tip < 1 {
		return false
	}
	return !ShouldDeferTipBackfill(tip, bs.ContiguousRawHeight())
}
