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

func TestVerifyScriptP2SHCLTVP2PK(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x47
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	lockHeight := int64(700)
	redeem := buildCLTVP2PKRedeemScript(lockHeight, pubC)
	p2sh := p2shScriptPubKey(redeem)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{12}, PrevIdx: 0, Sequence: 0xffffffff}},
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
	_, inner, err := parseTimelockDropRedeem(redeem, opCheckLockTimeVerify)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := wire.CalcSignatureHashLegacy(inner, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
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

func TestRedeemScriptMetaCLTVP2PK(t *testing.T) {
	_, pub := secp256k1.PrivKeyFromBytes([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32})
	redeem := buildCLTVP2PKRedeemScript(100, pub.SerializeCompressed())
	meta := RedeemScriptMeta(redeem)
	if meta["dogego_script_template"] != "cltv_pubkey" {
		t.Fatalf("%v", meta)
	}
	if meta["dogego_timelock"] != "absolute" {
		t.Fatalf("%v", meta)
	}
}
