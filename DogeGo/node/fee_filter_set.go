// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "sync"

// FeeFilterSet tracks BIP133 feefilter values per peer (Core aggregates max for mempoolminfee hints).
type FeeFilterSet struct {
	mu  sync.Mutex
	by  map[string]uint64
}

func NewFeeFilterSet() *FeeFilterSet {
	return &FeeFilterSet{by: make(map[string]uint64)}
}

func (s *FeeFilterSet) ensureMap() {
	if s == nil {
		return
	}
	if s.by == nil {
		s.by = make(map[string]uint64)
	}
}

// Set records a peer's minimum relay feerate (koinu/kB). Pass empty addr for a single anonymous peer.
func (s *FeeFilterSet) Set(addr string, rate uint64) {
	if s == nil {
		return
	}
	if addr == "" {
		addr = "@"
	}
	s.mu.Lock()
	s.ensureMap()
	s.by[addr] = rate
	s.mu.Unlock()
}

// Remove drops a peer's filter when the session ends.
func (s *FeeFilterSet) Remove(addr string) {
	if s == nil || addr == "" {
		return
	}
	s.mu.Lock()
	if s.by != nil {
		delete(s.by, addr)
	}
	s.mu.Unlock()
}

// Max returns the highest feefilter seen from any connected peer (0 when none).
func (s *FeeFilterSet) Max() uint64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var max uint64
	for _, r := range s.by {
		if r > max {
			max = r
		}
	}
	return max
}

// Load is an alias for Max (used where legacy atomic.Load was wired).
func (s *FeeFilterSet) Load() uint64 {
	return s.Max()
}

// For returns the feefilter from addr, or 0 when unset.
func (s *FeeFilterSet) For(addr string) uint64 {
	if s == nil || addr == "" {
		return 0
	}
	s.mu.Lock()
	rate := s.by[addr]
	s.mu.Unlock()
	return rate
}
