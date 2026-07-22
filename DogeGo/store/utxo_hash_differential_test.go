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

// TestUtxoSerializedHashLinearVsRebuild ensures Core hash_serialized is stable across
// incremental ApplyBlockRaw and a full RebuildFromChain replay (Milestone C scaffold).
func TestUtxoSerializedHashLinearVsRebuild(t *testing.T) {
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

	h1 := append([]byte(nil), blockRaw[:80]...)
	copy(h1[4:36], hash[:])
	h1[76] ^= 0x11
	raw1 := MakeTestBlockRaw(t, h1)
	h1Stored := append([]byte(nil), raw1[:80]...)
	id1 := pow.BlockHashLE(h1Stored)
	if err := j.AppendHeaders([][]byte{h1Stored}); err != nil {
		t.Fatal(err)
	}
	if err := raw.Put(id1, raw1); err != nil {
		t.Fatal(err)
	}

	linear := NewUtxoCache()
	for h := int64(0); h <= 1; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			t.Fatal(err)
		}
		id := pow.BlockHashLE(h80)
		body, err := raw.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if err := linear.ApplyBlockRaw(body, h); err != nil {
			t.Fatal(err)
		}
	}
	wantHash := linear.SerializedHashAtTip(j)

	rebuilt := NewUtxoCache()
	if err := rebuilt.RebuildFromChain(j, raw, 0, 1); err != nil {
		t.Fatal(err)
	}
	gotHash := rebuilt.SerializedHashAtTip(j)
	if gotHash != wantHash {
		t.Fatalf("hash_serialized mismatch: linear %s rebuild %s", wantHash, gotHash)
	}
	if linear.Count() != rebuilt.Count() {
		t.Fatalf("count linear=%d rebuild=%d", linear.Count(), rebuilt.Count())
	}
}

// TestUtxoSerializedHashReorgReplay simulates header rewind + UTXO replay: after building a
// three-block tip, resetting and replaying from genesis must restore the same hash_serialized.
func TestUtxoSerializedHashReorgReplay(t *testing.T) {
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

	type link struct {
		nonceXor byte
	}
	headers := [][]byte{append([]byte(nil), blockRaw[:80]...)}
	bodies := [][]byte{blockRaw}
	prevID := hash
	for _, link := range []link{{0x11}, {0x22}} {
		h := append([]byte(nil), headers[len(headers)-1]...)
		copy(h[4:36], prevID[:])
		h[76] ^= link.nonceXor
		body := MakeTestBlockRaw(t, h)
		hStored := append([]byte(nil), body[:80]...)
		id := pow.BlockHashLE(hStored)
		if err := j.AppendHeaders([][]byte{hStored}); err != nil {
			t.Fatal(err)
		}
		if err := raw.Put(id, body); err != nil {
			t.Fatal(err)
		}
		headers = append(headers, hStored)
		bodies = append(bodies, body)
		prevID = id
	}

	applyAll := func(u *UtxoCache) {
		t.Helper()
		for h := int64(0); h < int64(len(bodies)); h++ {
			if err := u.ApplyBlockRaw(bodies[h], h); err != nil {
				t.Fatal(err)
			}
		}
	}

	full := NewUtxoCache()
	applyAll(full)
	wantTip2 := full.SerializedHashAtTip(j)
	wantCount2 := full.Count()

	// Rewind UTXO to genesis and replay through height 2 (as after TruncateChainToHeight + RebuildUtxoThrough).
	full.Reset()
	if err := full.RebuildFromChain(j, raw, 0, 2); err != nil {
		t.Fatal(err)
	}
	if got := full.SerializedHashAtTip(j); got != wantTip2 {
		t.Fatalf("replay tip2 hash: got %s want %s", got, wantTip2)
	}
	if full.Count() != wantCount2 {
		t.Fatalf("replay tip2 count: got %d want %d", full.Count(), wantCount2)
	}

	// Partial replay through height 1, then extend to 2 again.
	partial := NewUtxoCache()
	if err := partial.RebuildFromChain(j, raw, 0, 1); err != nil {
		t.Fatal(err)
	}
	wantTip1 := partial.SerializedHashAtTip(j)
	if wantTip1 == wantTip2 {
		t.Fatal("expected distinct hash_serialized at heights 1 and 2")
	}
	if err := partial.ApplyBlockRaw(bodies[2], 2); err != nil {
		t.Fatal(err)
	}
	if got := partial.SerializedHashAtTip(j); got != wantTip2 {
		t.Fatalf("extend to tip2: got %s want %s", got, wantTip2)
	}
}
