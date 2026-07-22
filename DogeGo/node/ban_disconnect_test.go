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

func TestDisconnectAllRelaysKeepsPrimary(t *testing.T) {
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 8}, mustTestnetParams(t), "/DogeGo/", net.Dialer{})
	pm.mu.Lock()
	pm.sessions["10.0.0.1:22556"] = &peerLink{addr: "10.0.0.1:22556", primary: true}
	pm.sessions["10.0.0.2:22556"] = &peerLink{addr: "10.0.0.2:22556"}
	pm.primary = "10.0.0.1:22556"
	pm.mu.Unlock()
	pm.DisconnectAllRelays()
	if !pm.HasSession("10.0.0.1:22556") {
		t.Fatal("primary removed")
	}
	if pm.HasSession("10.0.0.2:22556") {
		t.Fatal("relay still connected")
	}
}
