// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"testing"
)

func TestDisconnectPeerNotFound(t *testing.T) {
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 8}, mustTestnetParams(t), "/DogeGo/", net.Dialer{})
	pm.mu.Lock()
	pm.sessions["10.0.0.1:22556"] = &peerLink{addr: "10.0.0.1:22556"}
	pm.mu.Unlock()
	err := pm.DisconnectPeer("10.0.0.99:22556")
	if err == nil || err.Error() != "Node not found in connected nodes" {
		t.Fatalf("err %v", err)
	}
}

func TestDisconnectPeerHostMatch(t *testing.T) {
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 8}, mustTestnetParams(t), "/DogeGo/", net.Dialer{})
	pm.mu.Lock()
	pm.sessions["127.0.0.1:22556"] = &peerLink{addr: "127.0.0.1:22556"}
	pm.mu.Unlock()
	if err := pm.DisconnectPeer("127.0.0.1:22556"); err != nil {
		t.Fatal(err)
	}
	if pm.HasSession("127.0.0.1:22556") {
		t.Fatal("still connected")
	}
}
