// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/binary"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

// Live mainnet batches can be a few minutes ahead of local nTime after DigiShield; must not rewind.
func TestMaybeRewindStaleHeaderTimes_postDigishieldSmallGap(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	const tipH = int64(150_000) // post-DigiShield (145000)
	prevHash := pow.BlockHashLE(g80[:])
	prev := append([]byte(nil), g80[:]...)
	baseTime := uint32(1_400_000_000)
	const batchHeaders = 500
	batch := make([]byte, 0, batchHeaders*80)
	for h := int64(1); h <= tipH; h++ {
		hdr := append([]byte(nil), prev...)
		copy(hdr[4:36], prevHash[:])
		hdr[76] ^= byte(h)
		binary.LittleEndian.PutUint32(hdr[68:72], baseTime+uint32(h*60))
		batch = append(batch, hdr...)
		prevHash = pow.BlockHashLE(hdr)
		prev = hdr
		if len(batch) >= batchHeaders*80 {
			if err := j.AppendWireHeaderBatch(batch); err != nil {
				t.Fatal(err)
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		if err := j.AppendWireHeaderBatch(batch); err != nil {
			t.Fatal(err)
		}
	}
	tip80, _ := j.ReadHeaderAt(tipH)
	tipTime := binary.LittleEndian.Uint32(tip80[68:72])
	peer := append([]byte(nil), prev...)
	copy(peer[4:36], prevHash[:])
	peer[76] ^= 0x42
	binary.LittleEndian.PutUint32(peer[68:72], tipTime+300) // +300s, below 1h stale floor
	rewound, err := maybeRewindStaleHeaderTimes(j, nil, p, []wire.DecodedHeader{{Header80: peer}}, nil)
	if rewound || err != nil {
		t.Fatalf("rewound=%v err=%v want no rewind for +300s on post-digishield tip", rewound, err)
	}
}
