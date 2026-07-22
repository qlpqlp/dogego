// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"testing"

	"dogego/wire"
)

func TestTryResolveMempoolRBFConflictsFullRBF(t *testing.T) {
	const parentVal = int64(200_000_000)
	parentHash := [32]byte{9}
	parentID := txidDisplayFromLE(parentHash)
	view := fixedPrevOutView{
		rpcOutpointKey(parentID, 0): {Value: parentVal, PkScript: []byte{0x51}},
	}
	old := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: 0xffffffff}}, // not RBF
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
	err := TryResolveMempoolRBFConflicts(newTx, pool, view, false)
	if !errors.Is(err, ErrRBFNotReplaceable) {
		t.Fatalf("opt-in only: %v", err)
	}
	if err := TryResolveMempoolRBFConflicts(newTx, pool, view, true); err != nil {
		t.Fatal(err)
	}
	if len(pool.raw) != 0 {
		t.Fatal("expected conflict removed")
	}
}
