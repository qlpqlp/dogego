// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/wire"
)

func TestWalletConflictsStored(t *testing.T) {
	oldID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	newID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	paths := &DataPaths{
		WalletConflictsForTx: func(txid string) []string {
			if txid == oldID {
				return []string{newID}
			}
			return nil
		},
	}
	conflicts := walletConflicts("", paths, nil, nil, nil, oldID)
	if len(conflicts) != 1 || conflicts[0] != newID {
		t.Fatalf("conflicts %#v", conflicts)
	}
}

func TestWalletTxSharesInput(t *testing.T) {
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 2e8, PkScript: []byte{0x51}}},
	}
	txA := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 1e8, PkScript: []byte{0x51}}},
	}
	txB := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 9e7, PkScript: []byte{0x52}}},
	}
	spent := walletTxInputKeys(txA)
	if !walletTxSharesInput(txB, spent) {
		t.Fatal("txB should share input with txA")
	}
	if walletTxSharesInput(parent, spent) {
		t.Fatal("parent should not share spend with txA inputs")
	}
}
