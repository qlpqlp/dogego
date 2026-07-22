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

func TestPQCarrierP2SHTxIsStandard(t *testing.T) {
	carrier := BuildPQCarrierP2SHScriptPubKey()
	tx := &wire.Tx{
		Version: 2,
		Vin: []wire.TxIn{{
			PrevHash: [32]byte{1},
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 1e8, PkScript: carrier}},
	}
	if err := IsStandardTx(tx, DefaultStandardPolicy(), DefaultMinRelayTxFeePerKB); err != nil {
		t.Fatal(err)
	}
}

func TestMempoolAdmissionAcceptsPQCommitmentStandard(t *testing.T) {
	commit, err := BuildPQCommitmentScript(PQTagDilithium, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 0, PkScript: commit}},
	}
	adm := MempoolAdmission{}
	if err := adm.CheckStandard(tx); err != nil {
		t.Fatal(err)
	}
}

func TestMempoolAdmissionAcceptsPQCarrierP2SHStandard(t *testing.T) {
	carrier := BuildPQCarrierP2SHScriptPubKey()
	tx := &wire.Tx{
		Version: 2,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{2}, PrevIdx: 1, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1e8, PkScript: carrier}},
	}
	adm := MempoolAdmission{}
	if err := adm.CheckStandard(tx); err != nil {
		t.Fatal(err)
	}
}

func TestPQCommitmentScriptIsStandard(t *testing.T) {
	commit := make([]byte, 32)
	script, err := BuildPQCommitmentScript(PQTagDilithium, commit)
	if err != nil {
		t.Fatal(err)
	}
	if !IsStandardScript(script, DefaultStandardPolicy()) {
		t.Fatal("canonical PQ OP_RETURN should be standard")
	}
}

func TestPQCommitmentTxIsStandard(t *testing.T) {
	commit := make([]byte, 32)
	pqScript, err := BuildPQCommitmentScript(PQTagFalcon, commit)
	if err != nil {
		t.Fatal(err)
	}
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout: []wire.TxOut{
			{Value: 0, PkScript: pqScript},
		},
	}
	if err := IsStandardTx(tx, DefaultStandardPolicy(), DefaultMinRelayTxFeePerKB); err != nil {
		t.Fatal(err)
	}
}
