// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"testing"

	"dogego/chain"
)

func TestPeerMgrSetMaxConnections(t *testing.T) {
	s, _ := ParseP2PMode("both", 8, 12)
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(s, p, "/DogeGo/", net.Dialer{})
	if err := pm.SetMaxConnections(7); err == nil {
		t.Fatal("expected error below minimum")
	}
	if err := pm.SetMaxConnections(16); err != nil {
		t.Fatal(err)
	}
	pm.mu.Lock()
	got := pm.p2p.MaxOutbound
	pm.mu.Unlock()
	if got != 16 {
		t.Fatalf("max outbound = %d", got)
	}
}
