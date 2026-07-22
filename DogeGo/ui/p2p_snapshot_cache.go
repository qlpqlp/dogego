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
	cp := make(map[string]any, len(s))
	for k, v := range s {
		cp[k] = v
	}
	p2pSnapCache.mu.Lock()
	p2pSnapCache.s = cp
	p2pSnapCache.at = time.Now()
	p2pSnapCache.mu.Unlock()
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
