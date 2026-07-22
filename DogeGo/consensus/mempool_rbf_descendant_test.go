// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"testing"

	"dogego/mempool"
	"dogego/wire"
)

// TestRBFConflictChildUnderpayRejects ensures a replacement that beats the conflict alone but
// underpays the conflict+child descendant package is rejected (Core PaysForRBF polarity).
func TestRBFConflictChildUnderpayRejects(t *testing.T) {
	pk := []byte{0x51}
	confirmed := [32]byte{0xab}
	confID := txidDisplayFromLE(confirmed)
	const confVal = int64(10_000_000_000) // 100 DOGE
	confirmedView := fixedPrevOutView{
		rpcOutpointKey(confID, 0): {Value: confVal, PkScript: pk},
	}

	conflict := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: confirmed, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 9_900_000_000, PkScript: pk}}, // fee 100M
	}
	child := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: conflict.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 9_000_000_000, PkScript: pk}}, // fee 900M from conflict output
	}
	// Replacement beats conflict fee (100M) but not conflict+child (~1B).
	newTx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: confirmed, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 9_700_000_000, PkScript: pk}}, // fee 300M
	}

	pool := mempool.New(10)
	for _, tx := range []*wire.Tx{conflict, child} {
		raw, err := tx.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		if err := pool.Add(raw); err != nil {
			t.Fatal(err)
		}
	}
	view := MultiPrevOutView([]PrevOutView{&MempoolPrevOutView{Pool: pool}, confirmedView})
	err := TryResolveMempoolRBFConflicts(newTx, pool, view, false)
	if !errors.Is(err, ErrRBFInsufficientFee) {
		t.Fatalf("expected ErrRBFInsufficientFee, got %v", err)
	}
	if pool.Count() != 2 {
		t.Fatalf("pool should keep conflict cluster, count=%d", pool.Count())
	}
}

// TestRBFConflictIgnoresUnconfirmedParent ensures ancestor fees are not charged against the replacement.
func TestRBFConflictIgnoresUnconfirmedParent(t *testing.T) {
	pk := []byte{0x51}
	confirmed := [32]byte{0xcd}
	confID := txidDisplayFromLE(confirmed)
	const confVal = int64(10_000_000_000)
	confirmedView := fixedPrevOutView{
		rpcOutpointKey(confID, 0): {Value: confVal, PkScript: pk},
	}

	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: confirmed, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 9_000_000_000, PkScript: pk}}, // fee 1B
	}
	conflict := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: parent.TxHash(),
			PrevIdx:  0,
			Sequence: wire.MaxBIP125RBFSequence,
		}},
		Vout: []wire.TxOut{{Value: 8_900_000_000, PkScript: pk}}, // fee 100M
	}
	// Replacement spends parent's output (same as conflict). Fee 200M beats conflict 100M + incremental,
	// but would fail if ancestor package (parent 1B + conflict 100M) were charged.
	newTx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: parent.TxHash(),
			PrevIdx:  0,
			Sequence: wire.MaxBIP125RBFSequence,
		}},
		Vout: []wire.TxOut{{Value: 8_800_000_000, PkScript: pk}}, // fee 200M vs parent output 9B
	}

	pool := mempool.New(10)
	for _, tx := range []*wire.Tx{parent, conflict} {
		raw, err := tx.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		if err := pool.Add(raw); err != nil {
			t.Fatal(err)
		}
	}
	view := MultiPrevOutView([]PrevOutView{&MempoolPrevOutView{Pool: pool}, confirmedView})
	if err := TryResolveMempoolRBFConflicts(newTx, pool, view, false); err != nil {
		t.Fatalf("expected accept (parent not in conflict set): %v", err)
	}
	// Conflict removed; unconfirmed parent remains.
	parentID := txidDisplayFromLE(parent.TxHash())
	if _, err := pool.GetRawByTxID(parentID); err != nil {
		t.Fatal("parent should remain in mempool")
	}
	conflictID := txidDisplayFromLE(conflict.TxHash())
	if _, err := pool.GetRawByTxID(conflictID); err == nil {
		t.Fatal("conflict should be removed")
	}
}
