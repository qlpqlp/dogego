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

func TestVerifyScriptP2SHCLTVMultisig(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x43
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	ms := buildTestMultisigRedeem(1, pubC)
	lockHeight := int64(600)
	redeem := buildCLTVMultisigRedeemScript(lockHeight, ms)
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

	spend := &wire.Tx{
		Version:  1,
		LockTime: uint32(lockHeight),
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xfffffffe,
		}},
		Vout: []wire.TxOut{{Value: 4_000_000_000, PkScript: []byte{0x51}}},
	}
	digest, err := wire.CalcSignatureHashLegacy(ms, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(0x00)
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()

	view := &MempoolPrevOutView{Pool: pool}
	flags := ScriptFlagsForHeight(4_000_000, chain.MainnetDogecoin)
	if err := VerifyScriptFlags(spend, view, flags); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyScriptP2SHCSVMultisig(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x45
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	ms := buildTestMultisigRedeem(1, pubC)
	relSeq := int64(3)
	redeem := buildCSVMultisigRedeemScript(relSeq, ms)
	p2sh := p2shScriptPubKey(redeem)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{10}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5_000_000_000, PkScript: p2sh}},
	}
	pool := mempool.New(50)
	raw, _ := funding.Serialize()
	if err := pool.Add(raw); err != nil {
		t.Fatal(err)
	}

	spend := &wire.Tx{
		Version: 2,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: uint32(relSeq),
		}},
		Vout: []wire.TxOut{{Value: 4_000_000_000, PkScript: []byte{0x51}}},
	}
	digest, err := wire.CalcSignatureHashLegacy(ms, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(0x00)
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()

	view := &MempoolPrevOutView{Pool: pool}
	flags := ScriptFlagsForHeight(500_000, chain.MainnetDogecoin)
	if flags&ScriptVerifyCheckSequenceVerify == 0 {
		t.Fatal("expected CSV flag")
	}
	if err := VerifyScriptFlags(spend, view, flags); err != nil {
		t.Fatal(err)
	}
}

func TestParseTimelockMultisigRedeem(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x44
	_, pub := secp256k1.PrivKeyFromBytes(sec)
	ms := buildTestMultisigRedeem(1, pub.SerializeCompressed())
	redeem := buildCLTVMultisigRedeemScript(42, ms)
	lt, tail, err := parseTimelockMultisigRedeem(redeem, opCheckLockTimeVerify)
	if err != nil || lt != 42 || !bytes.Equal(tail, ms) {
		t.Fatalf("lt=%d tail=%x err=%v", lt, tail, err)
	}
	if !isCLTVMultisigRedeem(redeem) {
		t.Fatal("isCLTVMultisigRedeem")
	}
}
