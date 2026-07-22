// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/wire"
)

func TestTryResolveMempoolRBFConflicts(t *testing.T) {
	const parentVal = int64(200_000_000)
	parentID := txidDisplayFromLE([32]byte{9})
	view := fixedPrevOutView{
		rpcOutpointKey(parentID, 0): {Value: parentVal, PkScript: []byte{0x51}},
	}
	parentHash := [32]byte{9}
	old := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: []byte{0x51}}},
	}
	newTx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: []byte{0x51}}},
	}
	oldRaw, _ := old.Serialize()
	oldID := txidDisplayFromLE(old.TxHash())
	pool := &rbfMockPool{
		spend: map[string]string{rpcOutpointKey(parentID, 0): oldID},
		raw:   map[string][]byte{oldID: oldRaw},
	}
	// new pays 150M fee vs old 100M; must also beat 100M + incremental on new size
	if err := TryResolveMempoolRBFConflicts(newTx, pool, view, false); err != nil {
		t.Fatal(err)
	}
	if len(pool.raw) != 0 {
		t.Fatalf("expected conflict removed, still have %d", len(pool.raw))
	}
}
