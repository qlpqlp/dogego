// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestFeeFilterSetZeroValueSet(t *testing.T) {
	var s FeeFilterSet
	s.Set("p:1", 100)
	if s.Max() != 100 {
		t.Fatalf("zero value set: max=%d", s.Max())
	}
}

func TestFeeFilterSetMaxAndRemove(t *testing.T) {
	s := NewFeeFilterSet()
	s.Set("a:1", 100)
	s.Set("b:1", 250)
	if s.Max() != 250 {
		t.Fatalf("max = %d", s.Max())
	}
	s.Remove("b:1")
	if s.Max() != 100 {
		t.Fatalf("after remove max = %d", s.Max())
	}
	s.Remove("a:1")
	if s.Max() != 0 {
		t.Fatalf("empty max = %d", s.Max())
	}
}
