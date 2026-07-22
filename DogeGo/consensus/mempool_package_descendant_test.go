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

func TestCheckAdmissionDescendantLimitsOnChain(t *testing.T) {
	pool := mempool.New(100)
	var prevHash [32]byte
	prevHash[0] = 0xaa
	for i := 0; i < 25; i++ {
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: p2pkhScript()}},
		}
		if err := pool.Add(tx.SerializeForHash()); err != nil {
			t.Fatal(err)
		}
		prevHash = tx.TxHash()
	}
	sizes, _ := pool.BuildMempoolSizes()
	extra := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: p2pkhScript()}},
	}
	err := CheckMempoolPackageLimits(extra, pool, sizes, 25, 25, 101)
	if err == nil {
		t.Fatal("expected descendant limit on chain ancestor")
	}
}
