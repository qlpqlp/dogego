// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestTrySeedContiguousFromCheckpointSkipsAncientGap(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 12)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw); err != nil {
		t.Fatal(err)
	}
	// Height 1 only; gap at 2; bodies 3..10.
	for h := int64(1); h <= 1; h++ {
		writeFakeBody(t, j, rs, h)
	}
	for h := int64(3); h <= 10; h++ {
		writeFakeBody(t, j, rs, h)
	}
	params, _ := chain.ParamsFor(chain.RebootTestnet)
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(50)
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, utxo)

	bs.contiguousMu.Lock()
	bs.contiguousTip = -1
	bs.contiguousMu.Unlock()
	bs.recomputeContiguousTipLocked()
	if got := bs.ContiguousRawHeight(); got != 1 {
		t.Fatalf("recompute contiguous=%d want 1 (ancient gap at 2)", got)
	}
	if !bs.TrySeedContiguousFromCheckpoint(10) {
		t.Fatal("expected checkpoint seed at 10")
	}
	if got := bs.ContiguousRawHeight(); got != 10 {
		t.Fatalf("after checkpoint seed contiguous=%d want 10", got)
	}
	if got := bs.RampReplayContiguousFromDisk(); got != 10 {
		t.Fatalf("ramp contiguous=%d want 10 (still gap at 2)", got)
	}
}

func writeFakeBody(t *testing.T, j *store.HeaderJournal, rs *store.RawBlockStore, h int64) {
	t.Helper()
	hdr, err := j.ReadHeaderAt(h)
	if err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 200)
	copy(body[:80], hdr)
	hash := pow.BlockHashLE(hdr)
	if err := os.WriteFile(filepath.Join(rs.Dir(), hex.EncodeToString(hash[:])+".bin"), body, 0o600); err != nil {
		t.Fatal(err)
	}
}
