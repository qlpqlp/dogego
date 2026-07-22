// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strings"
	"sync"
	"time"

	"dogego/p2p"
)

// blockAssistCandidatesRefreshInterval is how often the main loop merges new learned addrs into assist dials.
const blockAssistCandidatesRefreshInterval = 90 * time.Second

// BlockAssistCandidates holds a dial pool refreshed during IBD (DNS seeds + learned_addrs + scorer history).
type BlockAssistCandidates struct {
	mu              sync.RWMutex
	addrs           []string
	wantBlockHeight int64 // >=0 prefer archival peers during refresh
}

// SetWantBlockHeight updates the block height used for archival peer ordering on Refresh (-1 = off).
func (c *BlockAssistCandidates) SetWantBlockHeight(h int64) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.wantBlockHeight = h
	c.mu.Unlock()
}

// NewBlockAssistCandidates builds the initial assist peer list.
func NewBlockAssistCandidates(discovered []string, scorer *BlockPeerScorer) *BlockAssistCandidates {
	c := &BlockAssistCandidates{}
	c.Refresh(discovered, nil, scorer)
	return c
}

// Refresh merges discovery results, optional addnode targets, PeerMgr learned addresses, and scorer history.
func (c *BlockAssistCandidates) Refresh(discovered []string, pm *PeerMgr, scorer *BlockPeerScorer, addedNodes ...[]string) {
	if c == nil {
		return
	}
	seen := make(map[string]struct{})
	var manual []string
	addManual := func(a string) {
		a = strings.TrimSpace(a)
		if a == "" {
			return
		}
		if _, ok := seen[a]; ok {
			return
		}
		seen[a] = struct{}{}
		manual = append(manual, a)
	}
	var rest []string
	addRest := func(a string) {
		a = strings.TrimSpace(a)
		if a == "" {
			return
		}
		if _, ok := seen[a]; ok {
			return
		}
		seen[a] = struct{}{}
		rest = append(rest, a)
	}
	for _, list := range addedNodes {
		for _, a := range list {
			addManual(a)
		}
	}
	for _, a := range discovered {
		addRest(a)
	}
	if pm != nil {
		for _, a := range pm.AddrPoolSnapshot() {
			addRest(a)
		}
	}
	tail := p2p.PreferIPv4First(SpreadHostPortsByGroup16(rest))
	base := make([]string, 0, len(manual)+len(tail))
	base = append(base, manual...)
	base = append(base, tail...)
	merged := base
	if scorer != nil {
		wantH := int64(-1)
		c.mu.Lock()
		wantH = c.wantBlockHeight
		c.mu.Unlock()
		merged = scorer.MergeDiscoveryCandidates(base, wantH)
		merged = prependPinnedAddrs(merged, manual)
	}
	c.mu.Lock()
	c.addrs = merged
	c.mu.Unlock()
}

// prependPinnedAddrs moves configured addnode (and other pinned) entries to the front after scorer merge.
func prependPinnedAddrs(merged, pinned []string) []string {
	if len(pinned) == 0 || len(merged) == 0 {
		return merged
	}
	seen := make(map[string]struct{}, len(pinned))
	var out []string
	for _, p := range pinned {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		for _, a := range merged {
			if a == p {
				out = append(out, p)
				seen[p] = struct{}{}
				break
			}
		}
	}
	for _, a := range merged {
		if _, ok := seen[a]; !ok {
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		return merged
	}
	return out
}

// RefreshBlockAssistPool updates want-height archival ordering then merges discovery into the pool.
func RefreshBlockAssistPool(c *BlockAssistCandidates, discovered []string, pm *PeerMgr, scorer *BlockPeerScorer, bs *BlockStoreCtx, addedNodes ...[]string) {
	if c == nil {
		return
	}
	c.SetWantBlockHeight(blockFetchWantHeight(bs))
	c.Refresh(discovered, pm, scorer, addedNodes...)
}

// Snapshot returns a copy of the current candidate list (safe for concurrent dial loops).
func (c *BlockAssistCandidates) Snapshot() []string {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]string(nil), c.addrs...)
}

// Len reports how many dial targets are in the pool (for logging / UI).
func (c *BlockAssistCandidates) Len() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.addrs)
}

// IBDProgressWithAssistPool copies raw IBD stats and adds assist_peer_pool when set.
func IBDProgressWithAssistPool(raw map[string]interface{}, pool *BlockAssistCandidates) map[string]interface{} {
	if pool == nil || len(raw) == 0 {
		return raw
	}
	out := make(map[string]interface{}, len(raw)+1)
	for k, v := range raw {
		out[k] = v
	}
	out["assist_peer_pool"] = pool.Len()
	return out
}

// IBDProgressWithDiscoveryFeed adds discovery_feed_size when a feed is present.
func IBDProgressWithDiscoveryFeed(raw map[string]interface{}, pool *BlockAssistCandidates, feed *PeerDiscoveryFeed) map[string]interface{} {
	raw = IBDProgressWithAssistPool(raw, pool)
	if feed == nil || len(raw) == 0 {
		return raw
	}
	out := make(map[string]interface{}, len(raw)+1)
	for k, v := range raw {
		out[k] = v
	}
	out["discovery_feed_size"] = feed.Len()
	return out
}
