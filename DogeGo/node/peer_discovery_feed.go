// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strings"
	"sync"

	"dogego/wire"
)

const maxDiscoveryFeedAddrs = 512

// PeerDiscoveryFeed accumulates host:ports from DNS discovery and inbound addr messages.
type PeerDiscoveryFeed struct {
	mu    sync.RWMutex
	seen  map[string]struct{}
	order []string
}

// NewPeerDiscoveryFeed seeds the feed with initial discovery results.
func NewPeerDiscoveryFeed(initial []string) *PeerDiscoveryFeed {
	f := &PeerDiscoveryFeed{seen: make(map[string]struct{})}
	for _, a := range initial {
		f.Note(a)
	}
	return f
}

// NoteFromAddrPayload decodes a P2P addr message and learns each host:port.
func (f *PeerDiscoveryFeed) NoteFromAddrPayload(pl []byte) int {
	if f == nil || len(pl) == 0 {
		return 0
	}
	addrs, err := wire.DecodeAddrPayload(pl)
	if err != nil {
		return 0
	}
	n := 0
	for _, a := range addrs {
		hp := a.HostPort()
		if hp == "" || !IsIPPortRoutable(a.IP, a.Port) {
			continue
		}
		f.Note(hp)
		n++
	}
	return n
}

// Note adds one address if not already present (FIFO cap).
func (f *PeerDiscoveryFeed) Note(addr string) {
	addr = strings.TrimSpace(addr)
	if f == nil || addr == "" || !IsHostPortRoutable(addr) {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.seen[addr]; ok {
		return
	}
	f.seen[addr] = struct{}{}
	f.order = append(f.order, addr)
	if len(f.order) > maxDiscoveryFeedAddrs {
		drop := f.order[0]
		f.order = f.order[1:]
		delete(f.seen, drop)
	}
}

// Snapshot returns all known addresses (newest retained up to maxDiscoveryFeedAddrs).
func (f *PeerDiscoveryFeed) Snapshot() []string {
	if f == nil {
		return nil
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return append([]string(nil), f.order...)
}

// DiscoverySnapshot returns feed addresses when non-empty, else fallback.
func DiscoverySnapshot(feed *PeerDiscoveryFeed, fallback []string) []string {
	if feed == nil {
		return fallback
	}
	snap := feed.Snapshot()
	if len(snap) > 0 {
		return snap
	}
	return fallback
}

// Len reports feed size.
func (f *PeerDiscoveryFeed) Len() int {
	if f == nil {
		return 0
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.order)
}
