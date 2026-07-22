// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"net"
	"testing"
)

func TestIsPrivateIPv4(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"192.168.1.10", true},
		{"10.0.0.5", true},
		{"172.16.3.1", true},
		{"8.8.8.8", false},
		{"127.0.0.1", false},
	}
	for _, c := range cases {
		if got := isPrivateIPv4(net.ParseIP(c.ip)); got != c.want {
			t.Fatalf("%s private=%v want %v", c.ip, got, c.want)
		}
	}
}

func TestP2PPortForNetwork(t *testing.T) {
	if p := p2pPortForNetwork("testnet"); p != 44556 {
		t.Fatalf("testnet port %d", p)
	}
	if p := p2pPortForNetwork("mainnet"); p != 22556 {
		t.Fatalf("mainnet port %d", p)
	}
}

func TestBuildLanPeerHint(t *testing.T) {
	h := BuildLanPeerHint("testnet")
	if h.P2PPort != 44556 {
		t.Fatalf("port %d", h.P2PPort)
	}
	if h.Note == "" {
		t.Fatal("expected note")
	}
	for i, ip := range h.LANIPv4 {
		if h.ShareTargets[i] != ip+":44556" {
			t.Fatalf("target %q want %q", h.ShareTargets[i], ip+":44556")
		}
	}
}
