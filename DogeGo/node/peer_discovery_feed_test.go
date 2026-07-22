// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"testing"
)

func TestPeerDiscoveryFeedCapAndDedupe(t *testing.T) {
	f := NewPeerDiscoveryFeed(nil)
	for i := 0; i < maxDiscoveryFeedAddrs+5; i++ {
		f.Note(fmtAddr(i))
	}
	if f.Len() != maxDiscoveryFeedAddrs {
		t.Fatalf("len %d want %d", f.Len(), maxDiscoveryFeedAddrs)
	}
	f.Note(fmtAddr(0))
	if f.Len() != maxDiscoveryFeedAddrs {
		t.Fatalf("dup grew len to %d", f.Len())
	}
}

func TestDiscoverySnapshotFallback(t *testing.T) {
	feedAddr := "93.184.216.1:22556"
	fallbackAddr := "8.8.8.8:22556"
	f := NewPeerDiscoveryFeed([]string{feedAddr})
	got := DiscoverySnapshot(f, []string{fallbackAddr})
	if len(got) != 1 || got[0] != feedAddr {
		t.Fatalf("got %v", got)
	}
	empty := NewPeerDiscoveryFeed(nil)
	got2 := DiscoverySnapshot(empty, []string{fallbackAddr})
	if len(got2) != 1 || got2[0] != fallbackAddr {
		t.Fatalf("fallback %v", got2)
	}
}

func fmtAddr(i int) string {
	// Public IPv4 (not RFC1918) so IsHostPortRoutable accepts the feed entry.
	return fmt.Sprintf("93.184.%d.%d:22556", (i/256)%256, i%256)
}
