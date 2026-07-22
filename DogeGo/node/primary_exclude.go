// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "sync"

// PrimaryExclude tracks the current main sync peer so block-assist workers avoid dialing it.
type PrimaryExclude struct {
	mu   sync.RWMutex
	addr string
}

// Set updates the address assist workers must not use as a block-download peer.
func (p *PrimaryExclude) Set(addr string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.addr = addr
	p.mu.Unlock()
}

// Addr returns the current primary peer address (empty if unset).
func (p *PrimaryExclude) Addr() string {
	if p == nil {
		return ""
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.addr
}
