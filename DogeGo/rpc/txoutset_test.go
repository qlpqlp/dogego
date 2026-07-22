// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestExecGetTxOutSetInfo(t *testing.T) {
	j := &memJournal{tip: 3, best: "abc", gen: "g", count: 4, hdrs: [][]byte{make([]byte, 80), make([]byte, 80), make([]byte, 80), make([]byte, 80)}}
	m, code, msg := execGetTxOutSetInfo(j, nil, nil, nil, nil)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if m["height"].(int64) != 3 {
		t.Fatalf("height %#v", m["height"])
	}
	if m["bestblock"].(string) == "" {
		t.Fatalf("bestblock %v", m["bestblock"])
	}
}
