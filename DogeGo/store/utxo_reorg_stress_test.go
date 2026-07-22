// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"path/filepath"
	"testing"

	"dogego/pow"
)

// TestUtxoSerializedHashReorgStressLoop verifies hash_serialized after many rewind/replay cycles
// (Milestone C: chainstate replay fidelity without Core undo files).
func TestUtxoSerializedHashReorgStressLoop(t *testing.T) {
	const chainLen = 12
	dir := t.TempDir()
	blockRaw, hash := TestMinimalBlock()
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), blockRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Put(hash, blockRaw); err != nil {
		t.Fatal(err)
	}

	bodies := [][]byte{blockRaw}
	prevID := hash
	for h := int64(1); h < chainLen; h++ {
		prev, _ := j.ReadHeaderAt(h - 1)
		hdr := append([]byte(nil), prev...)
		copy(hdr[4:36], prevID[:])
		hdr[76] ^= byte(h)
		body := MakeTestBlockRaw(t, hdr)
		stored := append([]byte(nil), body[:80]...)
		id := pow.BlockHashLE(stored)
		if err := j.AppendHeaders([][]byte{stored}); err != nil {
			t.Fatal(err)
		}
		if err := raw.Put(id, body); err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, body)
		prevID = id
	}

	wantHash := make([]string, chainLen)
	ref := NewUtxoCache()
	for h := int64(0); h < chainLen; h++ {
		if err := ref.ApplyBlockRaw(bodies[h], h); err != nil {
			t.Fatal(err)
		}
		wantHash[h] = ref.SerializedHashAtTip(j)
	}

	const rounds = 4
	for round := 0; round < rounds; round++ {
		for h := int64(0); h < chainLen; h++ {
			u := NewUtxoCache()
			if err := u.RebuildFromChain(j, raw, 0, h); err != nil {
				t.Fatalf("round %d rebuild 0..%d: %v", round, h, err)
			}
			if got := u.SerializedHashAtTip(j); got != wantHash[h] {
				t.Fatalf("round %d height %d: hash %s want %s", round, h, got, wantHash[h])
			}
		}
		for rewind := int64(chainLen - 2); rewind >= 1; rewind-- {
			u := NewUtxoCache()
			if err := u.RebuildFromChain(j, raw, 0, rewind); err != nil {
				t.Fatal(err)
			}
			for h := rewind + 1; h < chainLen; h++ {
				if err := u.ApplyBlockRaw(bodies[h], h); err != nil {
					t.Fatal(err)
				}
			}
			if got := u.SerializedHashAtTip(j); got != wantHash[chainLen-1] {
				t.Fatalf("round %d rewind %d: tip hash %s want %s", round, rewind, got, wantHash[chainLen-1])
			}
		}
	}
}
