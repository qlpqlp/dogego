// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import "testing"

func TestFilterRowsByScriptSet(t *testing.T) {
	u := NewUtxoCache()
	u.SetTipHeightForTest(10)
	walletScript := []byte{0x76, 0xa9, 0x14, 1}
	otherScript := []byte{0x76, 0xa9, 0x14, 2}
	var op1, op2 [36]byte
	op1[0] = 1
	op2[0] = 2
	u.AddUtxoForTest(op1, UtxoEntry{Value: 100, PkScript: walletScript, Height: 5})
	u.AddUtxoForTest(op2, UtxoEntry{Value: 200, PkScript: otherScript, Height: 6})

	set := map[string]struct{}{string(walletScript): {}}
	got := u.FilterRowsByScriptSet(set, 0)
	if len(got) != 1 {
		t.Fatalf("len=%d want 1", len(got))
	}
	if got[0].Value != 100 {
		t.Fatalf("value=%d want 100", got[0].Value)
	}
	total, n := u.SumRowsMatching(func(pk []byte) bool { return string(pk) == string(walletScript) })
	if n != 1 || total != 100 {
		t.Fatalf("sum=%d count=%d", total, n)
	}
}
