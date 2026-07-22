// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/store"
)

func TestSortFundCandidatesAvoidReuse(t *testing.T) {
	reusedScript := []byte{0x76, 0xa9, 0x14, 1}
	freshScript := []byte{0x76, 0xa9, 0x14, 2}
	paths := &DataPaths{
		WalletAvoidReuse: func() bool { return true },
		WalletIsScriptReused: func(pk []byte) bool {
			return len(pk) > 0 && pk[len(pk)-1] == 1
		},
	}
	candidates := []fundPick{
		{row: store.UtxoDumpRow{Value: 100, PkScript: reusedScript}},
		{row: store.UtxoDumpRow{Value: 50, PkScript: freshScript}},
		{row: store.UtxoDumpRow{Value: 200, PkScript: reusedScript}},
	}
	sortFundCandidates(paths, candidates)
	if string(candidates[0].row.PkScript) != string(freshScript) {
		t.Fatalf("fresh script should be first, got %x", candidates[0].row.PkScript)
	}
	if candidates[1].row.Value != 200 || candidates[2].row.Value != 100 {
		t.Fatalf("reused group should be largest-first: %+v", candidates)
	}
}
