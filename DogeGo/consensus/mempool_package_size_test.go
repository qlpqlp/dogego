// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/mempool"
	"dogego/wire"
)

func TestCheckMempoolPackageSizeLimitsAdmissionTx(t *testing.T) {
	pool := mempool.New(100)
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff, Script: make([]byte, 900)}},
		Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: p2pkhScript()}},
	}
	if err := pool.Add(parent.SerializeForHash()); err != nil {
		t.Fatal(err)
	}
	child := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 49_000_000, PkScript: p2pkhScript()}},
	}
	ph := parent.TxHash()
	var k [36]byte
	copy(k[:32], ph[:])
	view := stubPrevOutView{k: {Value: parent.Vout[0].Value, PkScript: parent.Vout[0].PkScript}}
	err := CheckMempoolPackageSizeLimits(child, pool, view, 1, 101)
	if err == nil {
		t.Fatal("expected ancestor package size rejection")
	}
}

func p2pkhScript() []byte {
	pk := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
	return append(pk, 0x88, 0xac)
}
