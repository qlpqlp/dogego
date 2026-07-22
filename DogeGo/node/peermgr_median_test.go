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

func TestMedianInt32(t *testing.T) {
	if got := medianInt32(nil); got != 0 {
		t.Fatalf("empty got %d", got)
	}
	if got := medianInt32([]int32{10, -5, 20}); got != 10 {
		t.Fatalf("odd median got %d", got)
	}
	if got := medianInt32([]int32{-10, 10}); got != 0 {
		t.Fatalf("even median got %d", got)
	}
}

func TestPeerMgrMedianTimeOffset(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{Mode: P2PModeBoth, MaxOutbound: 4}, p, "/", net.Dialer{})
	pm.mu.Lock()
	pm.sessions["a:1"] = &peerLink{timeOffset: -100}
	pm.sessions["b:1"] = &peerLink{timeOffset: 200}
	pm.sessions["c:1"] = &peerLink{timeOffset: 50}
	pm.mu.Unlock()
	if got := pm.MedianTimeOffset(); got != 50 {
		t.Fatalf("median %d want 50", got)
	}
}
