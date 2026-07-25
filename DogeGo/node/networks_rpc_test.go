// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"testing"

	"dogego/p2p"
)

func TestBuildNetworksInfoIncludesOnion(t *testing.T) {
	p2p.ResetIPv6DialGateForTest()
	defer p2p.ResetIPv6DialGateForTest()
	rows := BuildNetworksInfo(P2PModeSettings{Mode: P2PModeCGNAT})
	if len(rows) < 3 {
		t.Fatalf("len %d", len(rows))
	}
	onion, ok := rows[2]["name"].(string)
	if !ok || onion != "onion" {
		t.Fatalf("onion %#v", rows[2])
	}
	if rows[2]["reachable"].(bool) {
		t.Fatal("onion should not be reachable")
	}
	if !rows[1]["reachable"].(bool) {
		t.Fatal("ipv6 should be reachable before dial gate")
	}
}

func TestBuildNetworksInfoIPv6UnreachableAfterGate(t *testing.T) {
	p2p.ResetIPv6DialGateForTest()
	defer p2p.ResetIPv6DialGateForTest()
	_ = p2p.ObserveDialError("[2001:db8::1]:44556", errors.New("connect: network is unreachable"))
	rows := BuildNetworksInfo(P2PModeSettings{})
	if rows[1]["reachable"].(bool) {
		t.Fatal("ipv6 should be unreachable after gate")
	}
}
