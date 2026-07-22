// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"testing"

	"dogego/wire"
)

func TestAddrRateLimitTokenBucket(t *testing.T) {
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 4}, mustTestnetParams(t), "/DogeGo/", net.Dialer{})
	pm.RegisterPrimary("93.184.216.1:22556", nil, nil, nil, nil)
	addrs := make([]wire.NetAddress, 0, 1005)
	for i := 0; i < 1005; i++ {
		addrs = append(addrs, wire.NetAddress{
			IP:   net.ParseIP("93.184.216." + itoa(i%200+1)),
			Port: 22556,
			Time: 1,
		})
	}
	pm.NoteAddrsFromPeer("93.184.216.1:22556", addrs)
	pm.mu.Lock()
	link := pm.sessions["93.184.216.1:22556"]
	proc := link.addrProcessed
	lim := link.addrRateLimited
	pm.mu.Unlock()
	if proc != 1000 {
		t.Fatalf("processed %d want 1000", proc)
	}
	if lim != 5 {
		t.Fatalf("rate limited %d want 5", lim)
	}
}

func TestAddrRateLimitWhitelistedAddnode(t *testing.T) {
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 4}, mustTestnetParams(t), "/DogeGo/", net.Dialer{})
	pm.SetPreferredPeers([]string{"93.184.216.1:22556"})
	pm.RegisterPrimary("93.184.216.1:22556", nil, nil, nil, nil)
	addrs := make([]wire.NetAddress, 0, 1005)
	for i := 0; i < 1005; i++ {
		addrs = append(addrs, wire.NetAddress{
			IP:   net.ParseIP("93.184.216." + itoa(i%200+1)),
			Port: 22556,
			Time: 1,
		})
	}
	pm.NoteAddrsFromPeer("93.184.216.1:22556", addrs)
	pm.mu.Lock()
	link := pm.sessions["93.184.216.1:22556"]
	proc := link.addrProcessed
	lim := link.addrRateLimited
	pm.mu.Unlock()
	if proc != 1005 {
		t.Fatalf("processed %d want 1005", proc)
	}
	if lim != 0 {
		t.Fatalf("rate limited %d", lim)
	}
}
