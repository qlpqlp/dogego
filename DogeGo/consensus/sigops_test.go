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

func TestGetTransactionSigOpCostP2PKH(t *testing.T) {
	pkScript := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
	pkScript = append(pkScript, 0x88, 0xac)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000, PkScript: pkScript}},
	}
	if got := GetTransactionSigOpCost(tx, nil); got != 4 {
		t.Fatalf("P2PKH output sigop cost = %d, want 4", got)
	}
}

func TestCountSigOpsMultisigAccurate(t *testing.T) {
	redeem := buildTestMultisigRedeem(1, make([]byte, 33))
	if got := CountSigOps(redeem, true); got != 1 {
		t.Fatalf("1-of-1 multisig accurate sigops = %d, want 1", got)
	}
}

func TestCheckTxSigOpCostRejectsHeavyMultisig(t *testing.T) {
	// 20-sigop CHECKMULTISIG placeholder in non-accurate mode × 4 = 80 per output.
	var script []byte
	script = append(script, 0xae)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
	}
	for i := 0; i < 300; i++ {
		tx.Vout = append(tx.Vout, wire.TxOut{Value: 1_000_000, PkScript: script})
	}
	err := CheckTxSigOpCost(tx, nil)
	if !errors.Is(err, ErrTxSigops) {
		t.Fatalf("got %v", err)
	}
}
