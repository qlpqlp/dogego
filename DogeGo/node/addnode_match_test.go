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

func TestAddnodeMatchesSession(t *testing.T) {
	if !addnodeMatchesSession("127.0.0.1:22556", "127.0.0.1:22556") {
		t.Fatal("exact")
	}
	if addnodeMatchesSession("127.0.0.1:22556", "127.0.0.1:22557") {
		t.Fatal("port mismatch")
	}
}

func TestConnectedAddressesForAddedNode(t *testing.T) {
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 4}, mustTestnetParams(t), "/DogeGo/", net.Dialer{})
	pm.mu.Lock()
	pm.sessions["10.0.0.5:22556"] = &peerLink{addr: "10.0.0.5:22556"}
	pm.mu.Unlock()
	rows := pm.ConnectedAddressesForAddedNode("10.0.0.5:22556")
	if len(rows) != 1 {
		t.Fatalf("len %d", len(rows))
	}
	m, ok := rows[0].(map[string]interface{})
	if !ok || m["connected"] != "outbound" {
		t.Fatalf("%#v", rows[0])
	}
}

func TestConnectedAddressesForAddedNodeInbound(t *testing.T) {
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 4}, mustTestnetParams(t), "/DogeGo/", net.Dialer{})
	pm.mu.Lock()
	pm.sessions["10.0.0.5:22556"] = &peerLink{addr: "10.0.0.5:22556", inbound: true}
	pm.mu.Unlock()
	rows := pm.ConnectedAddressesForAddedNode("10.0.0.5:22556")
	m := rows[0].(map[string]interface{})
	if m["connected"] != "inbound" {
		t.Fatalf("%#v", m)
	}
}
