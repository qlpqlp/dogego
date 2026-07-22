// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"
	"testing"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/chain"
	"dogego/mempool"
	"dogego/wire"
)

func TestVerifyScriptNestedP2SHP2PKH(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x44
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	innerRedeem := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	innerRedeem = append(innerRedeem, 0x88, 0xac)
	forward := p2shScriptPubKey(innerRedeem)
	outerP2SH := p2shScriptPubKey(forward)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{11}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5_000_000_000, PkScript: outerP2SH}},
	}
	pool := mempool.New(50)
	raw, _ := funding.Serialize()
	if err := pool.Add(raw); err != nil {
		t.Fatal(err)
	}

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 4_000_000_000, PkScript: []byte{0x51}}},
	}
	digest, err := wire.CalcSignatureHashLegacy(innerRedeem, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	spend.Vin[0].Script, err = concatScriptPushes(sigBytes, pubC, innerRedeem, forward)
	if err != nil {
		t.Fatal(err)
	}

	view := &MempoolPrevOutView{Pool: pool}
	flags := ScriptFlagsForHeight(4_000_000, chain.MainnetDogecoin)
	if err := VerifyScriptFlags(spend, view, flags); err != nil {
		t.Fatal(err)
	}
}

func concatScriptPushes(parts ...[]byte) ([]byte, error) {
	var out []byte
	for _, p := range parts {
		if len(p) == 0 {
			out = append(out, 0x00)
			continue
		}
		if len(p) <= 75 {
			out = append(out, byte(len(p)))
			out = append(out, p...)
			continue
		}
		return nil, fmt.Errorf("push too large for test helper")
	}
	return out, nil
}
