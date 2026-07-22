// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import "testing"

func TestEffectiveP2PServicesCompactFilters(t *testing.T) {
	p := Params{NodeNetwork: NodeNetwork}
	base := EffectiveP2PServices(p, false, false)
	if base&ServiceCompactFilters != 0 {
		t.Fatal("filters off")
	}
	with := EffectiveP2PServices(p, true, false)
	if with&ServiceNetwork == 0 || with&ServiceCompactFilters == 0 {
		t.Fatalf("services %x", with)
	}
	relay := EffectiveP2PServices(p, false, true)
	if relay&ServiceDogeGoRelayCGNAT == 0 {
		t.Fatal("relay bit missing")
	}
	if !HasDogeGoRelayCGNAT(relay) {
		t.Fatal("HasDogeGoRelayCGNAT")
	}
}

func TestPeerLikelyHasBlockLimitedPruneBeforeNetwork(t *testing.T) {
	both := ServiceNetwork | ServiceNetworkLimited
	start := int32(6_233_574)
	if PeerLikelyHasBlock(both, start, 10_006) {
		t.Fatal("limited+network peer at tip 6.2M should not serve height 10006")
	}
	if !PeerLikelyHasBlock(both, start, 6_233_000) {
		t.Fatal("limited peer should serve blocks near its tip")
	}
	if !PeerLikelyHasBlock(ServiceNetwork, start, 10_006) {
		t.Fatal("full NODE_NETWORK without LIMITED should serve ancient blocks")
	}
}
