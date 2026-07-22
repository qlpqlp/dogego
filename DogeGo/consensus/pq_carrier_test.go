// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"testing"

	"dogego/wire"
)

func TestPQCarrierRedeemScript(t *testing.T) {
	redeem := BuildPQCarrierRedeemScript()
	if !IsPQCarrierRedeemScript(redeem) {
		t.Fatal("expected carrier redeem")
	}
	spk := BuildPQCarrierP2SHScriptPubKey()
	if !IsPQCarrierScriptPubKey(spk) {
		t.Fatalf("spk=%x", spk)
	}
	want := "9b402803555511d15d81207d3e2cb3e6bc365e0e"
	if got := hex.EncodeToString(spk[2:22]); got != want {
		t.Fatalf("carrier h160=%s want %s", got, want)
	}
}

func TestPQCarrierPartRoundTrip(t *testing.T) {
	full := make([]byte, 1200)
	for i := range full {
		full[i] = byte(i)
	}
	pkLen := 400
	hdr, err := BuildPQCarrierHDR8(0, 1, pkLen, len(full))
	if err != nil {
		t.Fatal(err)
	}
	chunks, err := SplitPQCarrierPartPayload(full, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	scriptSig, err := BuildPQCarrierPartScriptSig(PQCarrierTagFalconFull, hdr, chunks)
	if err != nil {
		t.Fatal(err)
	}
	part, err := ParsePQCarrierPartScriptSig(scriptSig)
	if err != nil {
		t.Fatal(err)
	}
	mat, err := ReassemblePQCarrierParts([]*PQCarrierPart{part})
	if err != nil {
		t.Fatal(err)
	}
	if len(mat.PK) != pkLen || len(mat.Sig) != len(full)-pkLen {
		t.Fatalf("pk=%d sig=%d", len(mat.PK), len(mat.Sig))
	}
}

func TestPQCarrierP2SHAccepted(t *testing.T) {
	redeem := BuildPQCarrierRedeemScript()
	spk := BuildPQCarrierP2SHScriptPubKey()
	tag := []byte(PQCarrierTagFalconFull)
	hdr, _ := BuildPQCarrierHDR8(0, 1, 10, 20)
	payload := []byte("12345678901234567890")
	chunks, _ := SplitPQCarrierPartPayload(payload, 0, 1)
	scriptSig, err := BuildPQCarrierPartScriptSig(PQCarrierTagFalconFull, hdr, chunks)
	if err != nil {
		t.Fatal(err)
	}
	tx := &wire.Tx{
		Version: 2,
		Vin: []wire.TxIn{{
			PrevHash: [32]byte{},
			PrevIdx:  0,
			Script:   scriptSig,
		}},
		Vout: []wire.TxOut{{Value: 0, PkScript: spk}},
	}
	view := &singlePrevOutView{out: PrevOut{Value: 1e8, PkScript: spk}}
	if err := VerifyScriptFlags(tx, view, 0); err != nil {
		t.Fatalf("carrier p2sh verify: %v tag=%q hdr=%x", err, tag, hdr)
	}
	_ = redeem
}

type singlePrevOutView struct {
	out PrevOut
}

func (v *singlePrevOutView) Lookup([32]byte, uint32) (PrevOut, bool) {
	return v.out, true
}

func TestReconstructTXBaseFromTXC(t *testing.T) {
	commit, _ := BuildPQCommitmentScript(PQTagFalcon, make([]byte, 32))
	carrier := BuildPQCarrierP2SHScriptPubKey()
	txc := &wire.Tx{
		Version: 2,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0}},
		Vout: []wire.TxOut{
			{Value: 5e8, PkScript: []byte{0x76, 0xa9, 0x14, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 0x88, 0xac}},
			{Value: 0, PkScript: commit},
			{Value: 1e8, PkScript: carrier},
		},
	}
	base, restored, err := ReconstructTXBaseFromTXC(txc)
	if err != nil {
		t.Fatal(err)
	}
	if restored != 1e8 || len(base.Vout) != 1 || base.Vout[0].Value != 6e8 {
		t.Fatalf("base=%+v restored=%d", base.Vout, restored)
	}
}
