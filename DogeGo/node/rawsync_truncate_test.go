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

func TestResetAfterChainTruncate_realignsProbe(t *testing.T) {
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
	for i := 0; i < 20; i++ {
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
	if err := rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw); err != nil {
		t.Fatal(err)
	}
	bs := &BlockStoreCtx{Journal: j, Raw: rs}
	var s progressiveRawState
	s.nextProbe = 15
	s.inFlight = map[int64][32]byte{10: {}}
	bs.OnChainTruncated = func(int64) {
		s.ResetAfterChainTruncate(bs)
	}
	if err := TruncateChainToHeight(j, nil, bs, 3); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if s.nextProbe > 4 {
		t.Fatalf("nextProbe %d want <=4 after truncate to 3", s.nextProbe)
	}
	if len(s.inFlight) != 0 {
		t.Fatalf("inFlight len %d want 0", len(s.inFlight))
	}
}

func TestResetInFlightForHeaderRewind_clearsLaneMetadata(t *testing.T) {
	var s progressiveRawState
	s.inFlight = map[int64][32]byte{2: {}}
	s.inFlightLane = map[int64]int{2: 1}
	s.laneAddr = map[int]string{1: "127.0.0.1:22556"}
	s.laneDownloadSince = map[int]time.Time{1: time.Now()}
	s.ResetInFlightForHeaderRewind()
	if len(s.inFlight) != 0 || len(s.inFlightLane) != 0 {
		t.Fatal("expected in-flight maps cleared")
	}
	if len(s.laneAddr) != 0 || len(s.laneDownloadSince) != 0 {
		t.Fatal("expected lane sync metadata cleared")
	}
}
