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

func TestNoteInboundPeerLearnedRoutable(t *testing.T) {
	pm := NewPeerMgr(P2PModeSettings{MaxInbound: 4}, mustTestnetParams(t), "/DogeGo/", net.Dialer{})
	pm.addrs = NewAddrBook()
	addr := "93.184.216.60:22556"
	pm.noteInboundPeerLearned(addr, &wire.DecodedVersion{Services: 1})
	pm.mu.Lock()
	rec := pm.addrs.by[addr]
	pm.mu.Unlock()
	if rec == nil {
		t.Fatal("expected addrbook entry")
	}
	if rec.Tried {
		t.Fatal("inbound should not mark tried")
	}
	if rec.Services != 1 {
		t.Fatalf("services %d", rec.Services)
	}
}

func TestNoteInboundPeerLearnedSkipsNonRoutable(t *testing.T) {
	pm := NewPeerMgr(P2PModeSettings{}, mustTestnetParams(t), "/DogeGo/", net.Dialer{})
	pm.addrs = NewAddrBook()
	pm.noteInboundPeerLearned("127.0.0.1:22556", nil)
	pm.mu.Lock()
	n := len(pm.addrs.by)
	pm.mu.Unlock()
	if n != 0 {
		t.Fatalf("loopback should not be learned, got %d entries", n)
	}
}
