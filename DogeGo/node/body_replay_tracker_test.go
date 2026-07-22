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

func TestBodyReplayTrackerCompletesOnce(t *testing.T) {
	var tracker bodyReplayTracker
	var synced int64
	syncFn := func(cont int64) { synced = cont }

	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(100)
	bs := &BlockStoreCtx{Utxo: utxo}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 50
	bs.contiguousMu.Unlock()

	tracker.seedWasAhead()
	tracker.noteContiguousAdvance(bs, utxo, 50, "", syncFn, nil)
	if synced != 0 {
		t.Fatalf("sync during replay want 0 got %d", synced)
	}

	bs.contiguousMu.Lock()
	bs.contiguousTip = 100
	bs.contiguousMu.Unlock()
	tracker.noteContiguousAdvance(bs, utxo, 100, "", syncFn, nil)
	if synced != 100 {
		t.Fatalf("sync on complete want 100 got %d", synced)
	}

	synced = 0
	tracker.noteContiguousAdvance(bs, utxo, 101, "", syncFn, nil)
	if synced != 0 {
		t.Fatal("should only complete once")
	}
}

func TestBodyReplayCompleteHook(t *testing.T) {
	var tracker bodyReplayTracker
	var hookCont int64
	hook := func(_ *BlockStoreCtx, _ *store.UtxoCache, cont int64) { hookCont = cont }

	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(100)
	bs := &BlockStoreCtx{Utxo: utxo}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 100
	bs.contiguousMu.Unlock()

	tracker.seedWasAhead()
	tracker.noteContiguousAdvance(bs, utxo, 100, "", nil, hook)
	if hookCont != 100 {
		t.Fatalf("hook cont=%d want 100", hookCont)
	}
}

func TestShouldPersistSyncCheckpointFinalStretch(t *testing.T) {
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(5000)
	bs := &BlockStoreCtx{Utxo: utxo}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 4970
	bs.contiguousMu.Unlock()
	if !shouldPersistSyncCheckpoint(4971, bs) {
		t.Fatal("want checkpoint every block in final 64 of replay")
	}
}
