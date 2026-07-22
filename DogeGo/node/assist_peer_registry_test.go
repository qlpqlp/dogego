// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestAssistPeerRegistry(t *testing.T) {
	r := NewAssistPeerRegistry()
	r.Register("1.2.3.4:22556", 2)
	if r.Count() != 1 {
		t.Fatalf("count %d", r.Count())
	}
	inUse := r.InUseAddrs()
	if len(inUse) != 1 || inUse[0] != "1.2.3.4:22556" {
		t.Fatalf("inUse %#v", inUse)
	}
	snap := r.Snapshot()
	if len(snap) != 1 || snap[0].Lane != 2 {
		t.Fatalf("snap %#v", snap)
	}
	r.Unregister("1.2.3.4:22556")
	if r.Count() != 0 {
		t.Fatal("expected empty")
	}
	if len(r.InUseAddrs()) != 0 {
		t.Fatal("expected no in-use addrs")
	}
}

func TestPreferUnusedAssistPeers(t *testing.T) {
	got := preferUnusedAssistPeers([]string{"a", "b", "c"}, []string{"b"})
	if len(got) != 3 || got[0] != "a" || got[1] != "c" || got[2] != "b" {
		t.Fatalf("got %#v", got)
	}
	if preferUnusedAssistPeers([]string{"a"}, []string{"a"})[0] != "a" {
		t.Fatal("single candidate unchanged when all busy")
	}
}
