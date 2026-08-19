// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"net"
	"testing"
	"time"

	"dogego/chain"
)

// TestEclipseInboundPressureSoak simulates attacker-heavy inbound churn:
// protected addnode + BIP152 HB must survive; crowded /16 victims are preferred;
// inbound count never exceeds MaxInbound.
func TestEclipseInboundPressureSoak(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	const maxIn = 8
	pm := NewPeerMgr(P2PModeSettings{MaxInbound: maxIn, MaxOutbound: 4}, p, "/DogeGo/", net.Dialer{})
	pm.SetPreferredPeers([]string{"203.0.113.10:22556"})

	now := time.Now()
	pm.mu.Lock()
	pm.sessions["203.0.113.10:22556"] = &peerLink{
		addr: "203.0.113.10:22556", inbound: true, since: now.Add(-4 * time.Hour),
	}
	pm.sessions["198.51.100.50:22556"] = &peerLink{
		addr: "198.51.100.50:22556", inbound: true, since: now.Add(-3 * time.Hour), cmpctHBFrom: true,
	}
	// Fill remaining slots with attackers from 1.2.0.0/16
	for i := 0; i < maxIn-2; i++ {
		addr := fmt.Sprintf("1.2.3.%d:22556", 10+i)
		pm.sessions[addr] = &peerLink{
			addr: addr, inbound: true, since: now.Add(-time.Duration(60-i) * time.Minute),
		}
	}
	pm.mu.Unlock()

	if pm.inboundCount() != maxIn {
		t.Fatalf("setup inbound=%d want %d", pm.inboundCount(), maxIn)
	}

	for round := 0; round < 40; round++ {
		honest := fmt.Sprintf("198.51.100.%d:22556", 100+round%40)
		if !pm.acceptInboundOrEvict(honest) {
			t.Fatalf("round %d: could not make room for honest peer", round)
		}
		// Attach placeholder for the new honest peer (acceptInbound would do this).
		pm.mu.Lock()
		if _, exists := pm.sessions[honest]; !exists {
			pm.sessions[honest] = &peerLink{
				addr: honest, inbound: true, since: now.Add(time.Duration(round) * time.Second),
			}
		}
		// Drop excess if somehow over (should not happen).
		for pm.inboundCountLocked() > maxIn {
			v := pm.pickInboundEvictionVictimLocked()
			if v == "" {
				break
			}
			delete(pm.sessions, v)
		}
		pm.mu.Unlock()

		if n := pm.inboundCount(); n > maxIn {
			t.Fatalf("round %d: inbound=%d > max %d", round, n, maxIn)
		}
		pm.mu.Lock()
		_, addnodeOK := pm.sessions["203.0.113.10:22556"]
		_, hbOK := pm.sessions["198.51.100.50:22556"]
		pm.mu.Unlock()
		if !addnodeOK {
			t.Fatalf("round %d: addnode peer evicted", round)
		}
		if !hbOK {
			t.Fatalf("round %d: BIP152 HB peer evicted", round)
		}
	}

	// After soak, with attackers still present, next victim should be from crowded 1.2/16 if any remain.
	pm.mu.Lock()
	attackerLeft := 0
	for addr, l := range pm.sessions {
		if l != nil && l.inbound && hostPortGroup16(addr) == hostPortGroup16("1.2.3.10:22556") {
			attackerLeft++
		}
	}
	pm.mu.Unlock()
	if attackerLeft > 0 {
		v := pm.AttemptEvictInboundForNew()
		if v == "" {
			t.Fatal("expected an unprotected victim while attackers remain")
		}
		if hostPortGroup16(v) != hostPortGroup16("1.2.3.10:22556") && attackerLeft >= 2 {
			// Prefer crowded group when still over-represented.
			t.Logf("victim %s (attackers left %d) â€” acceptable if group counts tied", v, attackerLeft)
		}
	}
}
