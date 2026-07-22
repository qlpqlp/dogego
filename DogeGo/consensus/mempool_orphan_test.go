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

func TestAcceptMempoolTxWithOrphansPromotesChild(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x99
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xee}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 2_000_000_000, PkScript: pkScript}},
	}
	fundRaw, _ := funding.Serialize()
	pool := mempool.New(10)
	_ = pool.Add(fundRaw)

	parent := signedP2PKHSpend(t, funding, pkScript, priv, pubC, 1_800_000_000)
	parentRaw, _ := parent.Serialize()
	child := signedP2PKHSpend(t, parent, pkScript, priv, pubC, 1_600_000_000)
	childRaw, _ := child.Serialize()

	orphans := mempool.NewOrphanPool(10)
	adm := MempoolAdmission{View: AdmissionPrevOutView(pool, nil, nil)}

	err := AcceptMempoolTxWithOrphans(childRaw, child, pool, orphans, adm, "")
	if !errors.Is(err, ErrOrphanTx) {
		t.Fatalf("child first: %v", err)
	}
	if orphans.Count() != 1 {
		t.Fatalf("orphan count %d", orphans.Count())
	}

	err = AcceptMempoolTxWithOrphans(parentRaw, parent, pool, orphans, adm, "")
	if err != nil {
		t.Fatal(err)
	}
	if pool.Count() != 3 {
		t.Fatalf("pool count %d want 3", pool.Count())
	}
	if orphans.Count() != 0 {
		t.Fatalf("orphan count %d want 0", orphans.Count())
	}
}

func signedP2PKHSpend(t *testing.T, funding *wire.Tx, pkScript []byte, priv *secp256k1.PrivateKey, pubC []byte, outVal int64) *wire.Tx {
	t.Helper()
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: outVal, PkScript: pkScript}},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	sig := ecdsa.Sign(priv, digest[:])
	spend.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)
	return spend
}
