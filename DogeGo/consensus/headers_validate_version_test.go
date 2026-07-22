// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestChainIDFromVersionUnsigned(t *testing.T) {
	// Dogecoin mainnet-style nVersion with merge-mined chain id 0x62 (98) in bits 16..29.
	if got := chainIDFromVersion(0x620042); got != 98 {
		t.Fatalf("chain id from 0x620042: got %d want 98", got)
	}
	u := uint32(0x90001234)
	signedShift := int32(u) >> 16
	unsignedShift := int32(u >> 16)
	if signedShift == unsignedShift {
		t.Fatalf("test vector 0x%x should differ signed vs unsigned >>16 (got %d vs %d)", u, signedShift, unsignedShift)
	}
	if chainIDFromVersion(u) != unsignedShift {
		t.Fatalf("chainIDFromVersion got %d want %d", chainIDFromVersion(u), unsignedShift)
	}
}

func TestIsLegacyVersionU(t *testing.T) {
	if !isLegacyVersionU(1) {
		t.Fatal("v1 legacy")
	}
	if !isLegacyVersionU(2) {
		t.Fatal("v2 chain0 legacy")
	}
	if isLegacyVersionU(0x620042) {
		t.Fatal("encoded chain id must not be legacy")
	}
}
