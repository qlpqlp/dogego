// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"testing"

	"dogego/rpc"
	"dogego/wire"
)

func TestPeerSyncedHeadersAndBlocks(t *testing.T) {
	p := mustTestnetParams(t)
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 4}, p, "/DogeGo/", net.Dialer{})
	dv := &wire.DecodedVersion{StartHeight: 100}
	pm.RegisterPrimary("10.0.0.1:22556", nil, nil, nil, dv)
	pm.NotePeerHeaders("10.0.0.1:22556", 500, "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899")
	pm.NotePeerBlockAt("10.0.0.1:22556", 480)
	rows := pm.PeerInfoMaps(nil, p, &PeerInfoContext{SyncedBlocks: 490})
	m := rows[0]
	if m["synced_headers"].(int64) != 500 {
		t.Fatalf("synced_headers %#v", m["synced_headers"])
	}
	if m["dogego_best_header_hash"].(string) == "" {
		t.Fatal("expected tip hash")
	}
	if m["synced_blocks"].(int64) != 480 {
		t.Fatalf("synced_blocks %#v", m["synced_blocks"])
	}
}

func TestDisconnectBannedRelay(t *testing.T) {
	ban := rpc.NewMemoryBanManager()
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 8, MaxInbound: 4}, mustTestnetParams(t), "/DogeGo/", net.Dialer{})
	pm.mu.Lock()
	pm.sessions["10.0.0.2:22556"] = &peerLink{addr: "10.0.0.2:22556", inbound: false}
	pm.sessions["10.0.0.3:22556"] = &peerLink{addr: "10.0.0.3:22556", primary: true}
	pm.mu.Unlock()
	_ = ban.SetBan("10.0.0.2", "add", 3600, false)
	n := pm.DisconnectBanned(ban.IsBanned)
	if n != 1 {
		t.Fatalf("disconnected %d", n)
	}
	if pm.HasSession("10.0.0.2:22556") {
		t.Fatal("relay should be gone")
	}
	if !pm.HasSession("10.0.0.3:22556") {
		t.Fatal("primary should remain")
	}
}
