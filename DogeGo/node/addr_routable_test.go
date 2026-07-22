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

func TestIsHostPortRoutable(t *testing.T) {
	if IsHostPortRoutable("127.0.0.1:22556") {
		t.Fatal("loopback should not be routable")
	}
	if IsHostPortRoutable("10.0.0.1:22556") {
		t.Fatal("RFC1918 should not be routable")
	}
	if !IsHostPortRoutable("8.8.8.8:22556") {
		t.Fatal("public IPv4 should be routable")
	}
	if IsHostPortRoutable("8.8.8.8:0") {
		t.Fatal("zero port should not be routable")
	}
}

func TestAddrGroup16(t *testing.T) {
	g := addrGroup16(net.ParseIP("1.2.3.4"))
	if g != "1.2/16" {
		t.Fatalf("group %q", g)
	}
}

func TestSpreadHostPortsByGroup16(t *testing.T) {
	in := []string{"1.2.0.1:22556", "1.2.0.2:22556", "1.2.0.3:22556", "8.8.8.8:22556"}
	got := SpreadHostPortsByGroup16(in)
	if len(got) != len(in) {
		t.Fatalf("len %d", len(got))
	}
	// Round-robin: first from 1.2/16, then 8.8/16, then rest of 1.2/16.
	if got[0] != "1.2.0.1:22556" || got[1] != "8.8.8.8:22556" || got[2] != "1.2.0.2:22556" || got[3] != "1.2.0.3:22556" {
		t.Fatalf("spread order %v", got)
	}
}

func TestSpreadHostPortsByGroup16PreservesSingle(t *testing.T) {
	if got := SpreadHostPortsByGroup16([]string{"9.9.9.9:1"}); len(got) != 1 || got[0] != "9.9.9.9:1" {
		t.Fatalf("%v", got)
	}
}

func TestNormalizeAddrSeenUnixAncient(t *testing.T) {
	const now = int64(1_700_000_000)
	ancient := now - addrStaleAfterSeconds - 1
	if got := normalizeAddrSeenUnix(ancient, now); got != now {
		t.Fatalf("ancient clamp got %d want %d", got, now)
	}
}

func TestNormalizeAddrSeenUnix(t *testing.T) {
	const now = int64(1_700_000_000)
	if got := normalizeAddrSeenUnix(0, now); got != now {
		t.Fatalf("zero seen → now, got %d", got)
	}
	future := now + addrMaxFutureOffsetSec + 1
	if got := normalizeAddrSeenUnix(future, now); got != now {
		t.Fatalf("future clamp got %d want %d", got, now)
	}
	past := now - 3600
	if got := normalizeAddrSeenUnix(past, now); got != past {
		t.Fatalf("past kept, got %d", got)
	}
}
