// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"testing"
	"time"

	"dogego/chain"
)

func TestEvictStaleRelayPeers(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 4}, p, "/DogeGo/", net.Dialer{})
	pm.RegisterPrimary("primary:1", nil, nil, nil, nil)
	pm.mu.Lock()
	pm.sessions["stale:1"] = &peerLink{
		addr: "stale:1", since: time.Now().Add(-1 * time.Hour),
		lastRecv: time.Now().Add(-2 * relayPeerMaxIdle),
	}
	pm.sessions["fresh:1"] = &peerLink{
		addr: "fresh:1", since: time.Now().Add(-10 * time.Minute),
		lastRecv: time.Now(),
	}
	pm.mu.Unlock()
	n := pm.EvictStaleRelayPeers()
	if n != 1 {
		t.Fatalf("evicted %d want 1", n)
	}
	pm.mu.Lock()
	_, staleOk := pm.sessions["stale:1"]
	_, freshOk := pm.sessions["fresh:1"]
	pm.mu.Unlock()
	if staleOk {
		t.Fatal("stale peer should be removed")
	}
	if !freshOk {
		t.Fatal("fresh peer should remain")
	}
}
