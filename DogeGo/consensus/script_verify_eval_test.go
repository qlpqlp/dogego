// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/mempool"
	"dogego/wire"
)

// TestVerifyScriptEvalSimpleTrue spends a non-template scriptPubKey (OP_1) via EvalScript fallback.
func TestVerifyScriptEvalSimpleTrue(t *testing.T) {
	pkScript := []byte{0x51} // OP_1
	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xab}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000, PkScript: pkScript}},
	}
	raw, err := funding.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	pool := mempool.New(10)
	if err := pool.Add(raw); err != nil {
		t.Fatal(err)
	}
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: funding.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 900_000, PkScript: []byte{0x51}}},
	}
	view := &MempoolPrevOutView{Pool: pool}
	if err := VerifyScript(spend, view); err != nil {
		t.Fatalf("eval fallback: %v", err)
	}
}

// TestVerifyScriptEvalP2PKUsesSamePathAsTemplate ensures template P2PK still works after eval fallback exists.
func TestVerifyScriptEvalP2PKUsesSamePathAsTemplate(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x33
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	pkScript := append([]byte{0x21}, pubC...)
	pkScript = append(pkScript, 0xac)
	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xcd}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 2_000_000, PkScript: pkScript}},
	}
	pool := mempool.New(10)
	raw, _ := funding.Serialize()
	_ = pool.Add(raw)
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: funding.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000, PkScript: []byte{0x51}}},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	sig := append(ecdsa.Sign(priv, digest[:]).Serialize(), byte(wire.SigHashAll))
	spend.Vin[0].Script = buildPushData(sig)
	view := &MempoolPrevOutView{Pool: pool}
	if err := VerifyScript(spend, view); err != nil {
		t.Fatal(err)
	}
}
