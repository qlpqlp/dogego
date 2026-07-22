// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import "testing"

func TestPoolPauseClear(t *testing.T) {
	p := New(10)
	if err := p.Add([]byte{1, 0, 0, 0, 1}); err != nil {
		t.Fatal(err)
	}
	if p.Count() != 1 {
		t.Fatalf("count %d", p.Count())
	}
	p.SetPaused(true)
	if err := p.Add([]byte{2, 0, 0, 0, 1}); err == nil {
		t.Fatal("expected pause reject")
	}
	if n := p.Clear(); n != 1 {
		t.Fatalf("cleared %d", n)
	}
	p.SetPaused(false)
	if err := p.Add([]byte{2, 0, 0, 0, 1}); err != nil {
		t.Fatal(err)
	}
}
