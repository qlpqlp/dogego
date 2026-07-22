// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"sort"

	"dogego/chain"
	"dogego/p2p"
)

// preferArchivalPeers sorts addrs so full NODE_NETWORK peers (and those known to serve wantHeight)
// rank ahead of NODE_NETWORK_LIMITED-only peers during early block IBD (Core block fetch policy).
func preferArchivalPeers(addrs []string, wantHeight int64, meta func(addr string) (services uint64, startHeight int32, ok bool)) []string {
	if wantHeight < 0 || len(addrs) <= 1 || meta == nil {
		return addrs
	}
	type row struct {
		addr  string
		score int
	}
	rows := make([]row, len(addrs))
	for i, a := range addrs {
		score := 0
		if svc, start, ok := meta(a); ok {
			if chain.PeerLikelyHasBlock(svc, start, wantHeight) {
				if chain.HasFullBlockRelay(svc) && svc&chain.ServiceNetworkLimited == 0 {
					score += 1000
				} else {
					score += 100
				}
			} else {
				score -= 500
			}
		} else if wantHeight < 500_000 {
			// Ancient block fetch: prefer peers with known NODE_NETWORK (pruned/unknown peers often return stubs).
			score -= 300
		}
		rows[i] = row{addr: a, score: score}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].score > rows[j].score
	})
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.addr
	}
	return out
}

func (s *BlockPeerScorer) peerMeta(addr string) (services uint64, startHeight int32, ok bool) {
	if s == nil || addr == "" {
		return 0, 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entries[addr]
	if e == nil || (e.services == 0 && e.startHeight == 0) {
		return 0, 0, false
	}
	return e.services, e.startHeight, true
}

// DialableOrderForBlock is DialableOrder with archival peer preference for wantHeight.
func (s *BlockPeerScorer) DialableOrderForBlock(addrs []string, exclude string, wantHeight int64) []string {
	ordered := s.OrderCandidates(addrs, exclude)
	if wantHeight < 0 {
		return ordered
	}
	ordered = preferArchivalPeers(ordered, wantHeight, s.peerMeta)
	return p2p.PreferIPv4First(ordered)
}
