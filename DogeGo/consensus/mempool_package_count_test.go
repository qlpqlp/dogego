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

func TestCheckMempoolPackageLimitsAdmissionAncestorCount(t *testing.T) {
	pool := mempool.New(100)
	var prev [32]byte
	prev[0] = 0xaa
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: p2pkhScript()}},
	}
	// No mempool parents: admission ancestor count is 0.
	if err := CheckMempoolPackageLimits(tx, pool, nil, 0, 0, 0); err != nil {
		t.Fatalf("solo tx: %v", err)
	}
	// Chain 26 parents in mempool.
	parentHash := prev
	for i := 0; i < 26; i++ {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: p2pkhScript()}},
		}
		if err := pool.Add(parent.SerializeForHash()); err != nil {
			t.Fatal(err)
		}
		parentHash = parent.TxHash()
	}
	child := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: p2pkhScript()}},
	}
	sizes, _ := pool.BuildMempoolSizes()
	err := CheckMempoolPackageLimits(child, pool, sizes, 25, 25, 101)
	if err == nil {
		t.Fatal("expected too many ancestors for admission tx")
	}
}
