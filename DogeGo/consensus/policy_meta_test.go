// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestEffectiveMempoolRelayLimitsDefaults(t *testing.T) {
	e := EffectiveMempoolRelayLimits(MempoolRelayLimits{})
	if e.LimitAncestorCount != DefaultMaxMempoolAncestors {
		t.Fatalf("ancestor count %d", e.LimitAncestorCount)
	}
	if e.LimitDescendantSizeKB != DefaultMaxMempoolDescendantSizeKB {
		t.Fatalf("desc size %d", e.LimitDescendantSizeKB)
	}
	m := MempoolPackagePolicyMap(MempoolRelayLimits{LimitAncestorCount: 10})
	if m["limitancestorcount"] != 10 {
		t.Fatalf("%v", m)
	}
	if m["limitdescendantcount"] != DefaultMaxMempoolDescendants {
		t.Fatalf("%v", m)
	}
}

func TestStandardPolicyMap(t *testing.T) {
	p := DefaultStandardPolicy()
	p.AcceptDataCarrier = false
	m := StandardPolicyMap(p)
	if m["acceptdatacarrier"] != false {
		t.Fatalf("%v", m)
	}
	if m["datacarriersize"] != MaxDatacarrierBytes {
		t.Fatalf("%v", m)
	}
}
