// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

// Regression: undersized raw stubs left on disk must not block forward IBD at the contiguous frontier.
func TestFinishBatchStubPurgeRemovesInadequateBody(t *testing.T) {
	dir := t.TempDir()
	blockRaw, hash := store.TestMinimalBlock()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), blockRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, blockRaw[:80], 2)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Put(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	bs.noteBlockStoredAt(0)
	h1, err := j.ReadHeaderAt(1)
	if err != nil {
		t.Fatal(err)
	}
	id1 := pow.BlockHashLE(h1)
	stubName := filepath.Join(rs.Dir(), hex.EncodeToString(id1[:])+".bin")
	stub := make([]byte, 130)
	copy(stub, h1)
	if err := os.WriteFile(stubName, stub, 0o600); err != nil {
		t.Fatal(err)
	}
	if store.HasStoredBodyAtHeight(j, rs, 1, p.Net) {
		t.Fatal("130B stub at height 1 should not count as stored body on mainnet")
	}
	raw := &progressiveRawState{inFlight: make(map[int64][32]byte)}
	claim := rawBatchClaim{heights: []int64{1}, hashes: [][32]byte{id1}, lo: 1, hi: 1}
	stubErr := errors.New("batch incomplete: 1/1 block(s) missing; rejected 1 undersized stub(s)")
	if raw.finishBatch(bs, claim, 0, stubErr) {
		t.Fatal("expected finishBatch false when nothing stored")
	}
	if _, err := os.Stat(stubName); !os.IsNotExist(err) {
		t.Fatal("stub file should be purged after failed batch")
	}
}
