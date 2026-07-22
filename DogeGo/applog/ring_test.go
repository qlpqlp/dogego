// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package applog

import (
	"fmt"
	"testing"
)

func TestRingCap(t *testing.T) {
	r := New(5)
	for i := 0; i < 10; i++ {
		r.Add("x", fmt.Sprintf("%d", i))
	}
	all := r.Snapshot()
	if len(all) != 5 {
		t.Fatalf("len %d want 5", len(all))
	}
	if all[len(all)-1].Msg != "9" {
		t.Fatalf("last msg %q want 9", all[len(all)-1].Msg)
	}
	if all[0].Msg != "5" {
		t.Fatalf("first msg %q want 5", all[0].Msg)
	}
}
