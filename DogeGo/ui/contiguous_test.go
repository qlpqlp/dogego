// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"testing"

	"dogego/rpc"
	"dogego/store"
)

func TestChainActiveHeightForAPIPrefersLiveUtxo(t *testing.T) {
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(6702)
	cfg := StartConfig{
		UtxoCache: func() *store.UtxoCache { return utxo },
		ContiguousRawHeight: func() int64 {
			return 54506
		},
		ChainIBDSync: func() rpc.ChainIBDSnapshot {
			return rpc.ChainIBDSnapshot{Blocks: 17056}
		},
	}
	got := chainActiveHeightForAPI(cfg, 6_335_103)
	if got != 6702 {
		t.Fatalf("live UTXO tip %d want 6702 (stale ChainIBDSync was 17056)", got)
	}
}

func TestChainActiveHeightForAPICapsUtxoAtContiguous(t *testing.T) {
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(60_000)
	cfg := StartConfig{
		UtxoCache:           func() *store.UtxoCache { return utxo },
		ContiguousRawHeight: func() int64 { return 54_506 },
	}
	got := chainActiveHeightForAPI(cfg, 6_335_103)
	if got != 54_506 {
		t.Fatalf("UTXO ahead of bodies: got %d want contiguous cap 54506", got)
	}
}
