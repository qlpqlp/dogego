// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"sync"
	"time"
)

var p2pSnapCache struct {
	mu sync.RWMutex
	at time.Time
	s  map[string]any
}

func storeP2PSnapshotCache(s map[string]any) {
	if s == nil {
		return
	}
	// Never let an RPC/dialing stub without ibd_progress wipe a recent snap that still has rates.
	p2pSnapCache.mu.Lock()
	defer p2pSnapCache.mu.Unlock()
	if s["ibd_progress"] == nil && p2pSnapCache.s != nil && p2pSnapCache.s["ibd_progress"] != nil &&
		time.Since(p2pSnapCache.at) < 2*time.Minute {
		merged := make(map[string]any, len(p2pSnapCache.s)+8)
		for k, v := range p2pSnapCache.s {
			merged[k] = v
		}
		for _, k := range []string{
			"connections_outbound", "connections_inbound", "connections_total",
			"connections_outbound_relay", "block_assist_connections", "dedicated_header_connections",
			"peer_dialing", "health", "health_message", "primary_peer", "wired",
			"dogego_sync_activity", "warming_up",
		} {
			if v, ok := s[k]; ok {
				merged[k] = v
			}
		}
		delete(merged, "from_disk_snapshot")
		s = merged
	}
	cp := make(map[string]any, len(s))
	for k, v := range s {
		cp[k] = v
	}
	p2pSnapCache.s = cp
	p2pSnapCache.at = time.Now()
}

func cachedP2PSnapshot(maxAge time.Duration) map[string]any {
	p2pSnapCache.mu.RLock()
	defer p2pSnapCache.mu.RUnlock()
	if p2pSnapCache.s == nil {
		return nil
	}
	if maxAge > 0 && time.Since(p2pSnapCache.at) > maxAge {
		return nil
	}
	cp := make(map[string]any, len(p2pSnapCache.s)+1)
	for k, v := range p2pSnapCache.s {
		cp[k] = v
	}
	return cp
}
