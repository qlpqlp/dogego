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

func TestRampReplayContiguousFromDiskBounded(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 20)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw); err != nil {
		t.Fatal(err)
	}
	params, _ := chain.ParamsFor(chain.RebootTestnet)
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(18)
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, utxo)
	bs.noteBlockStoredAt(0)
	for h := int64(1); h <= 15; h++ {
		hdr, _ := j.ReadHeaderAt(h)
		body := make([]byte, 200)
		copy(body[:80], hdr)
		hash := pow.BlockHashLE(hdr)
		if err := os.WriteFile(filepath.Join(rs.Dir(), hex.EncodeToString(hash[:])+".bin"), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 2
	bs.contiguousMu.Unlock()
	got := rampReplayContiguousFromDiskBounded(bs, 4)
	if got != 15 {
		t.Fatalf("contiguous=%d want 15", got)
	}
}
