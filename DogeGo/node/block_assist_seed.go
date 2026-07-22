// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"strconv"

	"dogego/applog"
	"dogego/chain"
	"dogego/p2p"
)

// seedBlockAssistCandidates builds an assist dial pool from known scores, fixed seeds, and discovery.
// Uses synchronous fixed seeds so body IBD can start before DNS returns (Core: fixed seeds are enough).
func seedBlockAssistCandidates(ctx context.Context, p chain.Params, bs *BlockStoreCtx, scorer *BlockPeerScorer, feed *PeerDiscoveryFeed, discovered []string) *BlockAssistCandidates {
	want := blockFetchWantHeight(bs)
	seen := make(map[string]struct{})
	var base []string
	add := func(a string) {
		if a == "" {
			return
		}
		if _, ok := seen[a]; ok {
			return
		}
		seen[a] = struct{}{}
		base = append(base, a)
	}
	if scorer != nil {
		for _, a := range scorer.KnownAddresses() {
			add(a)
		}
	}
	for _, a := range p.FixedPeers {
		add(a)
	}
	for _, a := range DiscoverySnapshot(feed, discovered) {
		add(a)
	}
	if len(base) == 0 {
		for _, a := range p2p.DiscoverAddresses(ctx, p, func(msg string) { applog.Line("net", msg) }) {
			add(a)
		}
	}
	if len(base) == 0 {
		return nil
	}
	c := NewBlockAssistCandidates(assistPeerCandidates(ctx, p, base, scorer, want), scorer)
	if c != nil && c.Len() > 0 {
		applog.Line("block", "block-assist peer pool seeded ("+strconv.FormatInt(int64(c.Len()), 10)+
			" candidates, want height "+strconv.FormatInt(want, 10)+")")
	}
	return c
}
