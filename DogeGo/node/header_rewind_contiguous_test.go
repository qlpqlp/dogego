// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"

	"dogego/store"
)

func TestShouldPreserveContiguousDuringBodyIBD(t *testing.T) {
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(5015)
	bs := &BlockStoreCtx{Utxo: utxo, Journal: &store.HeaderJournal{}}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 5000
	bs.contiguousMu.Unlock()
	if !ShouldPreserveContiguousCache(bs) {
		t.Fatal("want preserve during utxo-ahead replay")
	}
}

func TestMaybeResetContiguousPreservesDuringReplay(t *testing.T) {
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(5015)
	bs := &BlockStoreCtx{Utxo: utxo, Journal: &store.HeaderJournal{}}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 5000
	bs.contiguousMu.Unlock()
	MaybeResetContiguousAfterHeaderRewind(bs)
	if got := bs.ContiguousRawHeight(); got != 5000 {
		t.Fatalf("contiguous=%d want 5000 preserved", got)
	}
}

func TestResetContiguousTipPreservesDuringForwardIBD(t *testing.T) {
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(5015)
	bs := &BlockStoreCtx{Utxo: utxo, Journal: &store.HeaderJournal{}}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 5000
	bs.contiguousMu.Unlock()
	bs.ResetContiguousTip()
	if got := bs.ContiguousRawHeight(); got != 5000 {
		t.Fatalf("contiguous=%d want 5000 preserved during forward IBD", got)
	}
}
