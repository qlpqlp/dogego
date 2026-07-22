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

	"dogego/chain"
	"dogego/mempool"
	"dogego/wire"
)

func TestVerifyScriptP2SHCSV(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x55
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)

	relSeq := int64(2) // 2-block relative lock
	redeem := buildCSVP2PKHRedeemScript(relSeq, h160)
	p2sh := p2shScriptPubKey(redeem)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5_000_000_000, PkScript: p2sh}},
	}
	pool := mempool.New(50)
	raw, _ := funding.Serialize()
	if err := pool.Add(raw); err != nil {
		t.Fatal(err)
	}

	inner := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	inner = append(inner, 0x88, 0xac)
	spend := &wire.Tx{
		Version: 2,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: uint32(relSeq),
		}},
		Vout: []wire.TxOut{{Value: 4_000_000_000, PkScript: []byte{0x51}}},
	}
	digest, err := wire.CalcSignatureHashLegacy(inner, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	spend.Vin[0].Script = append(buildP2PKHScriptSig(sigBytes, pubC), append([]byte{byte(len(redeem))}, redeem...)...)

	view := &MempoolPrevOutView{Pool: pool}
	flags := ScriptFlagsForHeight(500_000, chain.MainnetDogecoin)
	if flags&ScriptVerifyCheckSequenceVerify == 0 {
		t.Fatal("expected CSV flag at height 500000")
	}
	if err := VerifyScriptFlags(spend, view, flags); err != nil {
		t.Fatal(err)
	}
}

func TestCheckSequenceRejectsHighOperand(t *testing.T) {
	tx := &wire.Tx{
		Version: 2,
		Vin:     []wire.TxIn{{Sequence: 2}},
	}
	if err := CheckSequence(tx, 0, 5); err == nil {
		t.Fatal("expected unsatisfied sequence")
	}
}

func TestCSVActiveAt(t *testing.T) {
	if !CSVActiveAt(419328, chain.MainnetDogecoin) {
		t.Fatal("CSV should be active at 419328 on mainnet")
	}
	if CSVActiveAt(419327, chain.MainnetDogecoin) {
		t.Fatal("CSV should not be active before 419328 on mainnet")
	}
}
