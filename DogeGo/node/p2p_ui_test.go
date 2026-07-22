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

func TestBuildP2PUISnapshotCGNAT(t *testing.T) {
	s, err := ParseP2PMode("cgnat", 8, 0)
	if err != nil {
		t.Fatal(err)
	}
	out := BuildP2PUISnapshot(s, nil, "1.2.3.4:22556", "1.2.3.4:22556", nil)
	if out["cgnat_mode"] != true {
		t.Fatalf("cgnat_mode: %#v", out["cgnat_mode"])
	}
	if out["listen_enabled"] != false {
		t.Fatalf("listen: %#v", out["listen_enabled"])
	}
	if out["health"] != "warming" {
		t.Fatalf("health with one peer: %#v", out["health"])
	}
}

func TestBuildP2PUISnapshotCGNATOk(t *testing.T) {
	s, err := ParseP2PMode("cgnat", 8, 0)
	if err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(s, p, "/DogeGo/", net.Dialer{})
	pm.RegisterPrimary("a:1", nil, nil, nil, nil)
	pm.mu.Lock()
	pm.sessions["b:2"] = &peerLink{addr: "b:2", inbound: false}
	pm.mu.Unlock()
	out := BuildP2PUISnapshot(s, pm, "a:1", "a:1", nil)
	if out["health"] != "ok" {
		t.Fatalf("health: %#v", out["health"])
	}
	if out["connections_total"] != 2 {
		t.Fatalf("total: %#v", out["connections_total"])
	}
	if out["addrbook_n_key_set"] != true {
		t.Fatalf("addrbook_n_key_set: %#v", out["addrbook_n_key_set"])
	}
	info, ok := out["addrman_info"].(map[string]interface{})
	if !ok || info["all"] == nil {
		t.Fatalf("addrman_info: %#v", out["addrman_info"])
	}
}

func TestBuildP2PUISnapshot_cmpctHBCounts(t *testing.T) {
	s, err := ParseP2PMode("both", 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(s, p, "/DogeGo/", net.Dialer{})
	pm.mu.Lock()
	pm.sessions["hb:1"] = &peerLink{addr: "hb:1", cmpctHBTo: true, cmpctHBFrom: true}
	pm.mu.Unlock()
	out := BuildP2PUISnapshot(s, pm, "", "(connecting…)", nil)
	if out["bip152_hb_to"] != 1 || out["bip152_hb_from"] != 1 {
		t.Fatalf("bip152 counts: %#v", out)
	}
	if out["bip152_hb_max"] != maxCmpctHBPeers {
		t.Fatalf("bip152_hb_max: %#v", out["bip152_hb_max"])
	}
}

func TestBuildP2PUISnapshotIBDAssistWithoutPrimary(t *testing.T) {
	s, err := ParseP2PMode("both", 12, 16)
	if err != nil {
		t.Fatal(err)
	}
	reg := NewAssistPeerRegistry()
	reg.Register("10.0.0.1:22556", 1)
	reg.Register("10.0.0.2:22556", 2)
	extras := P2PExtrasFromNode(reg, nil, -1, -1, 4, true, "10.0.0.3:22556")
	out := BuildP2PUISnapshot(s, nil, "", "(connecting via DNS seeds then fixed seeds…)", extras)
	if out["peer_dialing"] != false {
		t.Fatalf("peer_dialing: %#v", out["peer_dialing"])
	}
	if out["connections_outbound"] != 3 {
		t.Fatalf("connections_outbound: %#v want 3 (2 assist + dedicated header)", out["connections_outbound"])
	}
	if out["connections_total"] != 3 {
		t.Fatalf("connections_total: %#v", out["connections_total"])
	}
	if out["block_assist_connections"] != 2 {
		t.Fatalf("assist: %#v", out["block_assist_connections"])
	}
	if out["health"] != "ok" && out["health"] != "warming" {
		t.Fatalf("health: %#v msg=%v", out["health"], out["health_message"])
	}
}

func TestBuildP2PUISnapshotBothNoInboundHint(t *testing.T) {
	s, err := ParseP2PMode("both", 12, 16)
	if err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(s, p, "/DogeGo/", net.Dialer{})
	pm.RegisterPrimary("a:1", nil, nil, nil, nil)
	pm.mu.Lock()
	pm.sessions["b:2"] = &peerLink{addr: "b:2", inbound: false}
	pm.mu.Unlock()
	out := BuildP2PUISnapshot(s, pm, "a:1", "a:1", nil)
	if out["connections_inbound"] != 0 {
		t.Fatalf("inbound: %#v", out["connections_inbound"])
	}
	hint, _ := out["inbound_hint"].(string)
	if hint == "" {
		t.Fatal("expected inbound_hint for both mode with listen and zero inbound")
	}
}

func TestPeerDialingIndicator(t *testing.T) {
	reg := NewAssistPeerRegistry()
	reg.Register("10.0.0.1:22556", 0)
	extras := P2PExtrasFromNode(reg, nil, -1, -1, 0, false, "")
	if PeerDialingIndicator("", "(connecting…)", extras) {
		t.Fatal("assist active should clear dialing")
	}
	if !PeerDialingIndicator("", "(connecting…)", nil) {
		t.Fatal("no sync peers should be dialing")
	}
}
