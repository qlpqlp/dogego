// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package applog

import (
	"sync"
	"time"
)

// Entry is one line for the web debug log (Core-style console).
type Entry struct {
	T   string `json:"t"`
	Cat string `json:"cat"`
	Msg string `json:"msg"`
}

// Ring is a fixed-capacity FIFO of log lines (oldest dropped when full).
type Ring struct {
	mu   sync.Mutex
	maxN int
	buf  []Entry
}

// New returns an empty ring holding at most maxN entries (clamped to 1..100000).
func New(maxN int) *Ring {
	if maxN < 1 {
		maxN = 1
	}
	if maxN > 100000 {
		maxN = 100000
	}
	return &Ring{maxN: maxN}
}

// Add records one line (timestamp is time.Now).
func (r *Ring) Add(category, message string) {
	if r == nil {
		return
	}
	now := time.Now()
	e := Entry{
		T:   now.Format(time.RFC3339Nano),
		Cat: category,
		Msg: message,
	}
	r.mu.Lock()
	r.buf = append(r.buf, e)
	if len(r.buf) > r.maxN {
		r.buf = r.buf[len(r.buf)-r.maxN:]
	}
	r.mu.Unlock()
}

// Snapshot returns a copy of all buffered entries in chronological order.
func (r *Ring) Snapshot() []Entry {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	out := make([]Entry, len(r.buf))
	copy(out, r.buf)
	r.mu.Unlock()
	return out
}

// SnapshotTail returns the last n entries (or fewer), chronological.
func (r *Ring) SnapshotTail(n int) []Entry {
	all := r.Snapshot()
	if n <= 0 || len(all) <= n {
		return all
	}
	return all[len(all)-n:]
}
