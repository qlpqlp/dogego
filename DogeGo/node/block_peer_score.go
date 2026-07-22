// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"sort"
	"sync"
	"time"

	"dogego/p2p"
)

const (
	blockPeerCooldownDial      = 45 * time.Second
	blockPeerCooldownSession     = 3 * time.Minute
	blockPeerCooldownReject      = 10 * time.Minute
	blockPeerScoreSuccessWeight  = 10
	blockPeerScoreFailurePenalty = 25
)

type blockPeerEntry struct {
	score       int
	blocks      int64
	failures    int
	lastOK      time.Time
	cooldownTo  time.Time
	services    uint64
	startHeight int32
}

// BlockPeerScorer ranks dial targets for block download (Core-style prefer productive peers).
type BlockPeerScorer struct {
	mu      sync.Mutex
	entries map[string]*blockPeerEntry
	dirty   bool
}

func NewBlockPeerScorer() *BlockPeerScorer {
	return &BlockPeerScorer{entries: make(map[string]*blockPeerEntry)}
}

func (s *BlockPeerScorer) entry(addr string) *blockPeerEntry {
	e, ok := s.entries[addr]
	if !ok {
		e = &blockPeerEntry{}
		s.entries[addr] = e
	}
	return e
}

// NoteDialFailure records a failed TCP connect or handshake (short cooldown).
func (s *BlockPeerScorer) NoteDialFailure(addr string) {
	if s == nil || addr == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(addr)
	e.failures++
	e.score -= blockPeerScoreFailurePenalty / 2
	if e.score < -200 {
		e.score = -200
	}
	until := time.Now().Add(blockPeerCooldownDial)
	if until.After(e.cooldownTo) {
		e.cooldownTo = until
	}
	s.dirty = true
}

// NoteStubBlock records an undersized block payload from a peer (pruned/stub getdata response).
func (s *BlockPeerScorer) NoteStubBlock(addr string) {
	if s == nil || addr == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(addr)
	e.failures++
	e.score -= blockPeerScoreFailurePenalty * 2
	if e.score < -500 {
		e.score = -500
	}
	until := time.Now().Add(blockPeerCooldownReject)
	if until.After(e.cooldownTo) {
		e.cooldownTo = until
	}
	s.dirty = true
}

// NoteSessionFailure records EOF/timeout/reject during block fetch (longer cooldown).
func (s *BlockPeerScorer) NoteSessionFailure(addr string, hard bool) {
	if s == nil || addr == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(addr)
	e.failures++
	e.score -= blockPeerScoreFailurePenalty
	if e.score < -500 {
		e.score = -500
	}
	cd := blockPeerCooldownSession
	if hard {
		cd = blockPeerCooldownReject
	}
	until := time.Now().Add(cd)
	if until.After(e.cooldownTo) {
		e.cooldownTo = until
	}
	s.dirty = true
}

// NoteHeadersDelivered boosts peers that returned validated header batches.
func (s *BlockPeerScorer) NoteHeadersDelivered(addr string, n int) {
	if s == nil || addr == "" || n <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(addr)
	e.lastOK = time.Now()
	e.score += n * (blockPeerScoreSuccessWeight / 2)
	if e.score > 10000 {
		e.score = 10000
	}
	e.cooldownTo = time.Time{}
	s.dirty = true
}

// NotePeerHandshake records NODE_* services and start height from a successful version handshake.
func (s *BlockPeerScorer) NotePeerHandshake(addr string, services uint64, startHeight int32) {
	if s == nil || addr == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(addr)
	e.services = services
	e.startHeight = startHeight
	s.dirty = true
}

// NoteBlocksDelivered boosts peers that returned full block payloads.
func (s *BlockPeerScorer) NoteBlocksDelivered(addr string, n int) {
	if s == nil || addr == "" || n <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(addr)
	e.blocks += int64(n)
	e.lastOK = time.Now()
	e.score += n * blockPeerScoreSuccessWeight
	if e.score > 10000 {
		e.score = 10000
	}
	e.cooldownTo = time.Time{}
	s.dirty = true
}

// BlockPeerStats is a snapshot of scorer state for one address (RPC / dashboard).
type BlockPeerStats struct {
	Score      int
	Blocks     int64
	Failures   int
	InCooldown bool
}

// Stats returns scorer data for addr when known.
func (s *BlockPeerScorer) Stats(addr string) (BlockPeerStats, bool) {
	if s == nil || addr == "" {
		return BlockPeerStats{}, false
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[addr]
	if !ok {
		return BlockPeerStats{}, false
	}
	return BlockPeerStats{
		Score:      e.score,
		Blocks:     e.blocks,
		Failures:   e.failures,
		InCooldown: !e.cooldownTo.IsZero() && now.Before(e.cooldownTo),
	}, true
}

// TopPeers returns up to n highest-scored addresses (any cooldown state).
func (s *BlockPeerScorer) TopPeers(n int) []struct {
	Addr   string
	Score  int
	Blocks int64
} {
	if s == nil || n <= 0 {
		return nil
	}
	type row struct {
		addr   string
		score  int
		blocks int64
	}
	s.mu.Lock()
	rows := make([]row, 0, len(s.entries))
	for addr, e := range s.entries {
		rows = append(rows, row{addr: addr, score: e.score, blocks: e.blocks})
	}
	s.mu.Unlock()
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		return rows[i].blocks > rows[j].blocks
	})
	if len(rows) > n {
		rows = rows[:n]
	}
	out := make([]struct {
		Addr   string
		Score  int
		Blocks int64
	}, len(rows))
	for i, r := range rows {
		out[i].Addr = r.addr
		out[i].Score = r.score
		out[i].Blocks = r.blocks
	}
	return out
}

// DialableOrder returns best-first peers, preferring addresses not in dial cooldown (Core addrman try).
// If every address is cooling down, the cooldown list is still returned so startup can proceed.
func (s *BlockPeerScorer) DialableOrder(addrs []string, exclude string) []string {
	ordered := s.OrderCandidates(addrs, exclude)
	if s == nil || len(ordered) == 0 {
		return ordered
	}
	now := time.Now()
	var ready, cooling []string
	s.mu.Lock()
	for _, a := range ordered {
		e := s.entries[a]
		if e == nil {
			ready = append(ready, a)
			continue
		}
		if !e.cooldownTo.IsZero() && now.Before(e.cooldownTo) {
			cooling = append(cooling, a)
		} else {
			ready = append(ready, a)
		}
	}
	s.mu.Unlock()
	if len(ready) > 0 {
		return p2p.PreferIPv4First(append(ready, cooling...))
	}
	return p2p.PreferIPv4First(cooling)
}

// OrderCandidates returns addrs sorted best-first, skipping exclude and peers in cooldown.
func (s *BlockPeerScorer) OrderCandidates(addrs []string, exclude string) []string {
	if len(addrs) == 0 {
		return nil
	}
	now := time.Now()
	type item struct {
		addr  string
		score int
		ok    bool
	}
	items := make([]item, 0, len(addrs))
	s.mu.Lock()
	for _, a := range addrs {
		if a == "" || a == exclude {
			continue
		}
		score := 0
		ok := true
		if e := s.entries[a]; e != nil {
			score = e.score
			ok = !(!e.cooldownTo.IsZero() && now.Before(e.cooldownTo))
		}
		items = append(items, item{addr: a, score: score, ok: ok})
	}
	s.mu.Unlock()
	sort.Slice(items, func(i, j int) bool {
		if items[i].ok != items[j].ok {
			return items[i].ok
		}
		return items[i].score > items[j].score
	})
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.addr)
	}
	return out
}

// CandidatesForWorker partitions ordered candidates across workers (round-robin on ranked list).
// wantBlockHeight >= 0 prefers full NODE_NETWORK peers for ancient block fetch (Core IBD).
func (s *BlockPeerScorer) CandidatesForWorker(all []string, exclude string, worker, nWorkers int, wantBlockHeight int64) []string {
	var ordered []string
	if wantBlockHeight >= 0 {
		ordered = s.DialableOrderForBlock(all, exclude, wantBlockHeight)
	} else {
		ordered = s.DialableOrder(all, exclude)
	}
	if nWorkers < 1 {
		nWorkers = 1
	}
	var out []string
	for i, addr := range ordered {
		if i%nWorkers == worker {
			out = append(out, addr)
		}
	}
	return out
}
