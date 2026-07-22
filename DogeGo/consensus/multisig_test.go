// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"testing"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/chain"
	"dogego/mempool"
	"dogego/wire"
)

func buildTestMultisigRedeem(nRequired int, pub []byte) []byte {
	var redeem []byte
	redeem = append(redeem, byte(0x50+nRequired))
	redeem = append(redeem, byte(len(pub)))
	redeem = append(redeem, pub...)
	redeem = append(redeem, byte(0x51), 0xae)
	return redeem
}

func TestVerifyScriptP2SHMultisig(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x77
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	redeem := buildTestMultisigRedeem(1, pubC)
	rh := hash160(redeem)
	p2sh := append([]byte{0xa9, 0x14}, rh[:]...)
	p2sh = append(p2sh, 0x87)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xab}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: p2sh}},
	}
	fundRaw, _ := funding.Serialize()
	pool := mempool.New(10)
	_ = pool.Add(fundRaw)

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 900_000_000, PkScript: []byte{0x51}}},
	}
	digest, _ := wire.CalcSignatureHashLegacy(redeem, wire.SigHashAll, spend, 0)
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(0x00) // CHECKMULTISIG dummy
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()

	view := AdmissionPrevOutView(pool, nil, nil)
	if err := VerifyScript(spend, view); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyScriptBareMultisig(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x79
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	redeem := buildTestMultisigRedeem(1, pubC)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xcd}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: redeem}},
	}
	fundRaw, _ := funding.Serialize()
	pool := mempool.New(10)
	_ = pool.Add(fundRaw)

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 900_000_000, PkScript: []byte{0x51}}},
	}
	digest, _ := wire.CalcSignatureHashLegacy(redeem, wire.SigHashAll, spend, 0)
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(0x00)
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	spend.Vin[0].Script = script.Bytes()

	view := AdmissionPrevOutView(pool, nil, nil)
	if err := VerifyScript(spend, view); err != nil {
		t.Fatal(err)
	}
}

func TestParseMultisigRedeemScript(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x88
	_, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	redeem := buildTestMultisigRedeem(1, pubC)
	n, keys, err := ParseMultisigRedeemScript(redeem)
	if err != nil || n != 1 || len(keys) != 1 {
		t.Fatalf("parse: n=%d keys=%d err=%v", n, len(keys), err)
	}
	if !IsMultisigRedeemScript(redeem) {
		t.Fatal("IsMultisigRedeemScript")
	}
}

func TestVerifyMultisigNullDummyMempoolPolicy(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x7a
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	redeem := buildTestMultisigRedeem(1, pubC)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xee}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: redeem}},
	}
	pool := mempool.New(10)
	fundRaw, _ := funding.Serialize()
	_ = pool.Add(fundRaw)

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 900_000_000, PkScript: []byte{0x51}}},
	}
	digest, _ := wire.CalcSignatureHashLegacy(redeem, wire.SigHashAll, spend, 0)
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(0x01) // nonzero dummy (invalid under NULLDUMMY)
	script.WriteByte(0x00)
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	spend.Vin[0].Script = script.Bytes()

	view := AdmissionPrevOutView(pool, nil, nil)
	flags := ScriptFlagsForMempool(0, chain.MainnetDogecoin, nil)
	if err := VerifyScriptFlags(spend, view, flags); err == nil || !bytes.Contains([]byte(err.Error()), []byte("NULLDUMMY")) {
		t.Fatalf("want SIG_NULLDUMMY, got %v", err)
	}
}
