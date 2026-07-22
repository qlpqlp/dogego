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

func TestAttemptEvictInboundForNew(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxInbound: 2, MaxOutbound: 4}, p, "/DogeGo/", net.Dialer{})
	pm.SetPreferredPeers([]string{"protect:22556"})
	pm.mu.Lock()
	pm.sessions["protect:22556"] = &peerLink{
		addr: "protect:22556", inbound: true, since: time.Now().Add(-2 * time.Hour),
	}
	pm.sessions["old:22556"] = &peerLink{
		addr: "old:22556", inbound: true, since: time.Now().Add(-1 * time.Hour),
	}
	pm.sessions["hb:22556"] = &peerLink{
		addr: "hb:22556", inbound: true, since: time.Now().Add(-3 * time.Hour), cmpctHBFrom: true,
	}
	pm.mu.Unlock()
	victim := pm.AttemptEvictInboundForNew()
	if victim != "old:22556" {
		t.Fatalf("victim %q want old:22556 (protect addnode + HB)", victim)
	}
}

func TestAttemptEvict_addnodeMatchesHostPort(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxInbound: 4, MaxOutbound: 4}, p, "/DogeGo/", net.Dialer{})
	// Preferred as IP; session may use same host:port — must protect via addnodeMatchesSession.
	pm.SetPreferredPeers([]string{"93.184.216.1:22556"})
	pm.mu.Lock()
	pm.sessions["93.184.216.1:22556"] = &peerLink{
		addr: "93.184.216.1:22556", inbound: true, since: time.Now().Add(-3 * time.Hour),
	}
	pm.sessions["198.51.100.9:22556"] = &peerLink{
		addr: "198.51.100.9:22556", inbound: true, since: time.Now().Add(-1 * time.Hour),
	}
	pm.mu.Unlock()
	victim := pm.AttemptEvictInboundForNew()
	if victim != "198.51.100.9:22556" {
		t.Fatalf("victim %q want unprotected peer", victim)
	}
}

func TestAttemptEvict_allProtectedReturnsEmpty(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxInbound: 2, MaxOutbound: 4}, p, "/DogeGo/", net.Dialer{})
	pm.SetPreferredPeers([]string{"10.0.0.1:22556"})
	pm.mu.Lock()
	pm.sessions["10.0.0.1:22556"] = &peerLink{
		addr: "10.0.0.1:22556", inbound: true, since: time.Now().Add(-2 * time.Hour),
	}
	pm.sessions["hb:22556"] = &peerLink{
		addr: "hb:22556", inbound: true, since: time.Now().Add(-3 * time.Hour), cmpctHBTo: true,
	}
	pm.mu.Unlock()
	if v := pm.AttemptEvictInboundForNew(); v != "" {
		t.Fatalf("want empty when all protected, got %q", v)
	}
}

func TestAttemptEvict_skipsOutboundAndPrimary(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxInbound: 4, MaxOutbound: 4}, p, "/DogeGo/", net.Dialer{})
	pm.mu.Lock()
	pm.primary = "primary:22556"
	pm.sessions["primary:22556"] = &peerLink{
		addr: "primary:22556", inbound: true, primary: true, since: time.Now().Add(-5 * time.Hour),
	}
	pm.sessions["out:22556"] = &peerLink{
		addr: "out:22556", inbound: false, since: time.Now().Add(-4 * time.Hour),
	}
	pm.sessions["in:22556"] = &peerLink{
		addr: "in:22556", inbound: true, since: time.Now().Add(-1 * time.Hour),
	}
	pm.mu.Unlock()
	if v := pm.AttemptEvictInboundForNew(); v != "in:22556" {
		t.Fatalf("victim %q want in:22556", v)
	}
}

func TestAttemptEvict_preferCrowdedNetgroup(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxInbound: 8, MaxOutbound: 4}, p, "/DogeGo/", net.Dialer{})
	now := time.Now()
	pm.mu.Lock()
	// Crowded 1.2.0.0/16 — three peers; oldest in that group should win over lone diverse peer
	// even if the diverse peer is older.
	pm.sessions["1.2.3.4:22556"] = &peerLink{
		addr: "1.2.3.4:22556", inbound: true, since: now.Add(-30 * time.Minute),
	}
	pm.sessions["1.2.3.5:22556"] = &peerLink{
		addr: "1.2.3.5:22556", inbound: true, since: now.Add(-20 * time.Minute),
	}
	pm.sessions["1.2.9.9:22556"] = &peerLink{
		addr: "1.2.9.9:22556", inbound: true, since: now.Add(-10 * time.Minute),
	}
	pm.sessions["198.51.100.1:22556"] = &peerLink{
		addr: "198.51.100.1:22556", inbound: true, since: now.Add(-2 * time.Hour),
	}
	pm.mu.Unlock()
	victim := pm.AttemptEvictInboundForNew()
	if victim != "1.2.3.4:22556" {
		t.Fatalf("victim %q want oldest in crowded /16 (1.2.3.4)", victim)
	}
}

func TestAcceptInboundOrEvict_makesRoom(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxInbound: 2, MaxOutbound: 4}, p, "/DogeGo/", net.Dialer{})
	pm.mu.Lock()
	pm.sessions["a:22556"] = &peerLink{addr: "a:22556", inbound: true, since: time.Now().Add(-2 * time.Hour)}
	pm.sessions["b:22556"] = &peerLink{addr: "b:22556", inbound: true, since: time.Now().Add(-1 * time.Hour)}
	pm.mu.Unlock()
	if !pm.acceptInboundOrEvict("c:22556") {
		t.Fatal("expected eviction to make room")
	}
	if pm.inboundCount() >= 2 {
		t.Fatalf("inbound count %d want < 2 after eviction", pm.inboundCount())
	}
	pm.mu.Lock()
	_, aOK := pm.sessions["a:22556"]
	_, bOK := pm.sessions["b:22556"]
	pm.mu.Unlock()
	if aOK {
		t.Fatal("oldest peer a should have been evicted")
	}
	if !bOK {
		t.Fatal("newer peer b should remain")
	}
}

func TestAcceptInboundOrEvict_rejectsWhenAllProtected(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxInbound: 2, MaxOutbound: 4}, p, "/DogeGo/", net.Dialer{})
	pm.SetPreferredPeers([]string{"10.0.0.1:22556", "10.0.0.2:22556"})
	pm.mu.Lock()
	pm.sessions["10.0.0.1:22556"] = &peerLink{addr: "10.0.0.1:22556", inbound: true, since: time.Now().Add(-2 * time.Hour)}
	pm.sessions["10.0.0.2:22556"] = &peerLink{addr: "10.0.0.2:22556", inbound: true, since: time.Now().Add(-1 * time.Hour)}
	pm.mu.Unlock()
	if pm.acceptInboundOrEvict("203.0.113.1:22556") {
		t.Fatal("should reject when all inbound slots are protected")
	}
	if pm.inboundCount() != 2 {
		t.Fatalf("count=%d", pm.inboundCount())
	}
}
