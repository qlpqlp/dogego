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

func TestPrimaryRedialCandidatesFixedPeer(t *testing.T) {
	got := primaryRedialCandidates(PrimaryRedialOpts{
		FixedPeer:   "1.2.3.4:22556",
		ExcludeAddr: "1.2.3.4:22556",
	})
	if len(got) != 1 || got[0] != "1.2.3.4:22556" {
		t.Fatalf("fixed peer redial = %v, want single fixed address", got)
	}
}

func TestPrimaryRedialCandidatesSpread(t *testing.T) {
	got := primaryRedialCandidates(PrimaryRedialOpts{
		Discovered: []string{"1.2.0.1:22556", "1.2.0.2:22556", "8.8.4.4:22556"},
	})
	if len(got) < 3 {
		t.Fatalf("got %v", got)
	}
	if got[0] != "1.2.0.1:22556" || got[1] != "8.8.4.4:22556" {
		t.Fatalf("spread order %v", got)
	}
}

func TestPrimaryRedialCandidatesExclude(t *testing.T) {
	s := NewBlockPeerScorer()
	got := primaryRedialCandidates(PrimaryRedialOpts{
		Discovered:  []string{"a:1", "b:2", "a:1"},
		ExcludeAddr: "a:1",
		Scorer:      s,
	})
	for _, a := range got {
		if a == "a:1" {
			t.Fatalf("excluded peer still in list: %v", got)
		}
	}
}

func TestPeerMgrReplacePrimary(t *testing.T) {
	s, _ := ParseP2PMode("both", 4, 8)
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(s, p, "/DogeGo/", net.Dialer{})
	pm.RegisterPrimary("old:1", nil, nil, nil, nil)
	pm.sessions["relay:1"] = &peerLink{addr: "relay:1", since: time.Now()}
	pm.order = []string{"old:1", "relay:1"}
	pm.ReplacePrimary("old:1", "new:2", nil, nil, nil, nil)
	if pm.primary != "new:2" {
		t.Fatalf("primary = %q", pm.primary)
	}
	if _, ok := pm.sessions["old:1"]; ok {
		t.Fatal("old primary session should be removed")
	}
	if l := pm.sessions["new:2"]; l == nil || !l.primary {
		t.Fatal("new primary session missing")
	}
	if _, ok := pm.sessions["relay:1"]; !ok {
		t.Fatal("relay session should remain")
	}
	if pm.order[0] != "new:2" {
		t.Fatalf("order = %v", pm.order)
	}
}
