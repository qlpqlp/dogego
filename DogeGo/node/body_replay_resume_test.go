// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestResumeAfterSnapshotReplayUnblocksForwardIBD(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(dir+"/headers.bin", genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, g80[:], 50)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw)
	params, _ := chain.ParamsFor(chain.RebootTestnet)
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(20)
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, utxo)
	bs.SeedContiguousTip(20)

	var raw progressiveRawState
	raw.mu.Lock()
	raw.idleFull = true
	raw.nextProbe = 21
	raw.mu.Unlock()

	raw.ResumeAfterSnapshotReplay(bs)

	raw.mu.Lock()
	idle := raw.idleFull
	raw.mu.Unlock()
	if idle {
		t.Fatal("expected idleFull=false when headers remain ahead of replay tip")
	}
}
