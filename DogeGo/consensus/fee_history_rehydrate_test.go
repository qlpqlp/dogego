// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/secp256k1"

	"dogego/mempool"
	"dogego/wire"
)

func TestRehydrateFromPoolTracksAdmission(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x55
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)

	funding := buildFundingTx(pkScript)
	fundRaw, _ := funding.Serialize()
	pool := mempool.New(64)
	if err := pool.Add(fundRaw); err != nil {
		t.Fatal(err)
	}
	spend := buildSignedSpend(t, funding, pkScript, priv, pubC, 900_000_000)
	spendRaw, _ := spend.Serialize()
	if err := pool.Add(spendRaw); err != nil {
		t.Fatal(err)
	}

	h := NewFeeHistory(8)
	view := AdmissionPrevOutView(pool, nil, nil)
	if n := h.RehydrateFromPool(pool, view, 100); n != 1 {
		t.Fatalf("rehydrated %d want 1", n)
	}
	if h.PendingMempoolFeeTracks() != 1 {
		t.Fatalf("pending %d", h.PendingMempoolFeeTracks())
	}
	if n := h.RehydrateFromPool(pool, view, 100); n != 0 {
		t.Fatalf("second rehydrate %d want 0", n)
	}
}

func buildFundingTx(pkScript []byte) *wire.Tx {
	return &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{3}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: pkScript}},
	}
}
