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

func TestIsFinalTx_heightLock(t *testing.T) {
	tx := &wire.Tx{
		Version:  1,
		LockTime: 100,
		Vin:      []wire.TxIn{{Sequence: wire.SequenceFinal - 1}},
		Vout:     []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	if IsFinalTx(tx, 99, 0) {
		t.Fatal("non-final sequence before locktime height")
	}
	if IsFinalTx(tx, 100, 0) {
		t.Fatal("still locked by sequence at locktime height")
	}
	tx.Vin[0].Sequence = wire.SequenceFinal
	if !IsFinalTx(tx, 100, 0) {
		t.Fatal("final at locktime height with final sequences")
	}
}

func TestCheckTxFinal_mempoolContext(t *testing.T) {
	j := &maturityJournal{coinHeight: 0, tip: 50}
	tx := &wire.Tx{
		Version:  1,
		LockTime: 52,
		Vin:      []wire.TxIn{{Sequence: wire.SequenceFinal - 1}},
		Vout:     []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	ctx, err := LockTimeContextForNextBlock(j, true)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.BlockHeight != 51 {
		t.Fatalf("height %d", ctx.BlockHeight)
	}
	if err := CheckTxFinal(tx, ctx); !errors.Is(err, ErrNonFinalTx) {
		t.Fatalf("want non-final, got %v", err)
	}
	tx.Vin[0].Sequence = wire.SequenceFinal
	tx.LockTime = 50
	if err := CheckTxFinal(tx, ctx); err != nil {
		t.Fatal(err)
	}
}
