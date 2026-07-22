// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"sync"
	"time"
)

// AssistPeerSnapshot describes an active block-assist TCP session (IBD worker).
type AssistPeerSnapshot struct {
	Addr      string
	Lane      int
	Since     time.Time
	BytesRecv uint64
	BytesSent uint64
}

type assistPeerEntry struct {
	AssistPeerSnapshot
	ctr *netByteCounter
}

// AssistPeerRegistry tracks block-assist download connections for RPC/UI.
type AssistPeerRegistry struct {
	mu     sync.Mutex
	active map[string]assistPeerEntry
}

func NewAssistPeerRegistry() *AssistPeerRegistry {
	return &AssistPeerRegistry{
		active: make(map[string]assistPeerEntry),
	}
}

// NetBytes returns cumulative recv/sent on block-assist TCP sessions.
func (r *AssistPeerRegistry) NetBytes() (recv, sent int64) {
	if r == nil {
		return 0, 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.active {
		if e.ctr == nil {
			continue
		}
		recv += int64(e.ctr.Recv())
		sent += int64(e.ctr.Sent())
	}
	return recv, sent
}

// Register starts tracking a block-assist session and returns its byte counter.
func (r *AssistPeerRegistry) Register(addr string, lane int) *netByteCounter {
	if r == nil || addr == "" {
		return nil
	}
	ctr := newNetByteCounter()
	r.mu.Lock()
	r.active[addr] = assistPeerEntry{
		AssistPeerSnapshot: AssistPeerSnapshot{Addr: addr, Lane: lane, Since: time.Now()},
		ctr:              ctr,
	}
	r.mu.Unlock()
	return ctr
}

func (r *AssistPeerRegistry) Unregister(addr string) {
	if r == nil || addr == "" {
		return
	}
	r.mu.Lock()
	delete(r.active, addr)
	r.mu.Unlock()
}

func (r *AssistPeerRegistry) Count() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	n := len(r.active)
	r.mu.Unlock()
	return n
}

// InUseAddrs returns peer addresses with an active block-assist session (one lane per addr).
func (r *AssistPeerRegistry) InUseAddrs() []string {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	out := make([]string, 0, len(r.active))
	for addr := range r.active {
		out = append(out, addr)
	}
	r.mu.Unlock()
	return out
}

func (r *AssistPeerRegistry) Snapshot() []AssistPeerSnapshot {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	out := make([]AssistPeerSnapshot, 0, len(r.active))
	for _, e := range r.active {
		s := e.AssistPeerSnapshot
		if e.ctr != nil {
			s.BytesRecv = e.ctr.Recv()
			s.BytesSent = e.ctr.Sent()
		}
		out = append(out, s)
	}
	r.mu.Unlock()
	return out
}
