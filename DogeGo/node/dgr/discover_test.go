// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import (
	"context"
	"testing"

	"dogego/chain"
)

func TestDiscoverTargetsMergesSources(t *testing.T) {
	ctx := context.Background()
	peers := []P2PRelayPeer{
		{TCPAddr: "203.0.113.10:22556", Services: chain.ServiceDogeGoRelayCGNAT | chain.ServiceNetwork},
		{TCPAddr: "198.51.100.1:22556", Services: chain.ServiceNetwork},
	}
	out := DiscoverTargets(ctx, "", []string{"seed.example:24433"}, nil, 24433, peers)
	if len(out) != 2 {
		t.Fatalf("targets %v", out)
	}
}

func TestNormalizeHostPort(t *testing.T) {
	if got := normalizeHostPort("relay.test", 24433); got != "relay.test:24433" {
		t.Fatal(got)
	}
	if got := normalizeHostPort("1.2.3.4:24433", 0); got != "1.2.3.4:24433" {
		t.Fatal(got)
	}
}

func TestSplitDNSSeedHosts(t *testing.T) {
	got := splitDNSSeedHosts("_a.example.com\n_b.example.com, _c.example.com")
	if len(got) != 3 {
		t.Fatalf("got %v", got)
	}
}

func TestEncodeDecodeRegister(t *testing.T) {
	payload := encodeRegister("testnet", "secret", 44556)
	net, tok, port, ok := decodeRegister(payload)
	if !ok || net != "testnet" || tok != "secret" || port != 44556 {
		t.Fatalf("got %q %q %d %v", net, tok, port, ok)
	}
}
