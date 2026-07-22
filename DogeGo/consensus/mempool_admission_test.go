// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"testing"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/mempool"
	"dogego/wire"
)

func TestCheckSpendConflictsMempoolDoubleSpend(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x55
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{3}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: pkScript}},
	}
	fundRaw, _ := funding.Serialize()
	pool := mempool.New(10)
	_ = pool.Add(fundRaw)

	spend1 := buildSignedSpend(t, funding, pkScript, priv, pubC, 900_000_000)
	spend1Raw, _ := spend1.Serialize()
	_ = pool.Add(spend1Raw)

	spend2 := buildSignedSpend(t, funding, pkScript, priv, pubC, 800_000_000)
	adm := MempoolAdmission{
		View: AdmissionPrevOutView(pool, nil, nil),
		Pool: pool,
	}
	err := adm.CheckSpendConflicts(spend2)
	if !errors.Is(err, ErrSpendInMempool) {
		t.Fatalf("got %v", err)
	}
}

func buildSignedSpend(t *testing.T, funding *wire.Tx, pkScript []byte, priv *secp256k1.PrivateKey, pubC []byte, outVal int64) *wire.Tx {
	t.Helper()
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: outVal, PkScript: []byte{0x51}}},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	sig := ecdsa.Sign(priv, digest[:])
	spend.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)
	return spend
}
