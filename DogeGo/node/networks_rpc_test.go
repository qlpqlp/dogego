// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestBuildNetworksInfoIncludesOnion(t *testing.T) {
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
}
