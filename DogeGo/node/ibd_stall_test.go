// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"path/filepath"
	"testing"
	"time"

	"dogego/pow"
	"dogego/store"
)

func TestMaybeRecoverIBDStallSkipsWhenRecentBlock(t *testing.T) {
	raw := &progressiveRawState{}
	raw.SetSyncParallelism(3)
	raw.mu.Lock()
	raw.ibdStarted = time.Now().Add(-30 * time.Minute)
	raw.lastStoredAt = time.Now().Add(-1 * time.Minute)
	raw.blocksStoredIBD = 100
	raw.mu.Unlock()

	var last time.Time
	before := time.Now()
	MaybeRecoverIBDStall(nil, nil, raw, nil, nil, nil, nil, nil, nil, &last, nil, nil, nil)
	if !last.IsZero() {
		t.Fatal("expected no recovery when block stored recently")
	}
	if time.Since(before) > time.Second {
		t.Fatal("should return quickly")
	}
}

func TestRealignProbeToLowestMissing(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		prev, _ := j.ReadHeaderAt(int64(i))
		h := append([]byte(nil), prev...)
		ph := pow.BlockHashLE(prev)
		copy(h[4:36], ph[:])
		h[76] ^= byte(i + 1)
		if err := j.AppendHeaders([][]byte{h}); err != nil {
			t.Fatal(err)
		}
	}
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	var s progressiveRawState
	s.nextProbe = 8
	bs := &BlockStoreCtx{Journal: j, Raw: rs}
	s.realignProbeToLowestMissing(bs)
	if s.nextProbe != 0 {
		t.Fatalf("nextProbe %d want 0", s.nextProbe)
	}
}
