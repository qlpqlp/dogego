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

func TestVerifyScriptP2SHCLTV(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x42
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)

	lockHeight := int64(500)
	redeem := buildCLTVP2PKHRedeemScript(lockHeight, h160)
	p2sh := p2shScriptPubKey(redeem)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{8}, PrevIdx: 0, Sequence: 0xffffffff}},
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
		Version:  1,
		LockTime: uint32(lockHeight),
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xfffffffe,
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
	flags := ScriptFlagsForHeight(4_000_000, chain.MainnetDogecoin)
	if err := VerifyScriptFlags(spend, view, flags); err != nil {
		t.Fatal(err)
	}
}

func TestCheckLockTimeRejectsLowTxLockTime(t *testing.T) {
	tx := &wire.Tx{
		LockTime: 100,
		Vin:      []wire.TxIn{{Sequence: 0xfffffffe}},
	}
	if err := CheckLockTime(tx, 0, 200); err == nil {
		t.Fatal("expected unsatisfied locktime")
	}
}

func p2shScriptPubKey(redeem []byte) []byte {
	h := hash160(redeem)
	return append(append([]byte{0xa9, 0x14}, h[:]...), 0x87)
}
