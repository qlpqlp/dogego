// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestMainnetFieldBlocksBundledStoreRoundTrip(t *testing.T) {
	gen := mustMainnetGenesis(t)
	byHeight := mainnetFieldBlockPayloads(t, 1, 3)
	if len(byHeight) < 3 {
		t.Skip("need field blocks 1-3 in testdata")
	}
	dir := t.TempDir()
	rs, err := store.OpenRawBlockStoreWithOpts(dir, store.BlockStorageOpts{Layout: store.BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	blocks := []struct {
		height int64
		raw    []byte
	}{
		{0, gen},
		{1, byHeight[1]},
		{2, byHeight[2]},
		{3, byHeight[3]},
	}
	for _, b := range blocks {
		id := pow.BlockHashLE(b.raw[:80])
		if err := rs.Put(id, b.raw); err != nil {
			t.Fatalf("put height %d: %v", b.height, err)
		}
		got, err := rs.Get(id)
		if err != nil {
			t.Fatalf("get height %d: %v", b.height, err)
		}
		if !bytes.Equal(got, b.raw) {
			t.Fatalf("height %d get mismatch", b.height)
		}
	}
	n, err := rs.Count()
	if err != nil || n != 4 {
		t.Fatalf("count=%d err=%v want 4", n, err)
	}
}

func TestMainnetFieldBlocksBundledStoreContiguous(t *testing.T) {
	gen := mustMainnetGenesis(t)
	byHeight := mainnetFieldBlockPayloads(t, 1, 3)
	if len(byHeight) < 3 {
		t.Skip("need field blocks 1-3 in testdata")
	}
	dir := t.TempDir()
	rs, err := store.OpenRawBlockStoreWithOpts(dir, store.BlockStorageOpts{Layout: store.BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Put(pow.BlockHashLE(gen[:80]), gen); err != nil {
		t.Fatal(err)
	}
	for h := int64(1); h <= 3; h++ {
		raw := byHeight[h]
		if err := rs.Put(pow.BlockHashLE(raw[:80]), raw); err != nil {
			t.Fatalf("put height %d: %v", h, err)
		}
	}
	tip, err := rs.ProbeBundledContiguousTip()
	if err != nil || tip != 3 {
		t.Fatalf("probe tip=%d err=%v want 3", tip, err)
	}
	for h := int64(0); h <= 3; h++ {
		want := gen
		if h > 0 {
			want = byHeight[h]
		}
		got, err := rs.GetByContiguousHeight(h)
		if err != nil {
			t.Fatalf("height %d: %v", h, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("height %d contiguous read mismatch", h)
		}
	}
}

func TestMainnetFieldBlocksBundledMeasureContiguous(t *testing.T) {
	gen := mustMainnetGenesis(t)
	byHeight := mainnetFieldBlockPayloads(t, 1, 3)
	if len(byHeight) < 3 {
		t.Skip("need field blocks 1-3 in testdata")
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), gen[:80])
	if err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStoreWithOpts(dir, store.BlockStorageOpts{Layout: store.BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Put(pow.BlockHashLE(gen[:80]), gen); err != nil {
		t.Fatal(err)
	}
	for h := int64(1); h <= 3; h++ {
		raw := byHeight[h]
		if err := j.AppendHeaders([][]byte{raw[:80]}); err != nil {
			t.Fatalf("header %d: %v", h, err)
		}
		if err := rs.Put(pow.BlockHashLE(raw[:80]), raw); err != nil {
			t.Fatalf("put %d: %v", h, err)
		}
	}
	measured := store.MeasureContiguousBodiesOnDisk(j, rs, chain.MainnetDogecoin, 0, 0)
	if measured != 3 {
		t.Fatalf("MeasureContiguousBodiesOnDisk=%d want 3", measured)
	}
	tip, err := rs.ProbeBundledContiguousTip()
	if err != nil || tip != 3 {
		t.Fatalf("probe tip=%d err=%v want 3", tip, err)
	}
}

func TestMainnetFieldBlocksBundledValidateStoredBodies(t *testing.T) {
	gen := mustMainnetGenesis(t)
	byHeight := mainnetFieldBlockPayloads(t, 1, 3)
	if len(byHeight) < 3 {
		t.Skip("need field blocks 1-3 in testdata")
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), gen[:80])
	if err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStoreWithOpts(dir, store.BlockStorageOpts{Layout: store.BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	ix, err := store.OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	rs.EnableTxIndexing(ix, true)
	if err := rs.Put(pow.BlockHashLE(gen[:80]), gen); err != nil {
		t.Fatal(err)
	}
	for h := int64(1); h <= 3; h++ {
		raw := byHeight[h]
		if err := j.AppendHeaders([][]byte{raw[:80]}); err != nil {
			t.Fatal(err)
		}
		if err := rs.Put(pow.BlockHashLE(raw[:80]), raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidateStoredBlockBodies(j, rs, ix, nil, chain.MainnetDogecoin, 0, 3); err != nil {
		t.Fatal(err)
	}
}

// TestCrashActiveBundledContiguous_MainnetFieldBlocks verifies torn bundled tails stop at last complete field block.
func TestCrashActiveBundledContiguous_MainnetFieldBlocks(t *testing.T) {
	gen := mustMainnetGenesis(t)
	byHeight := mainnetFieldBlockPayloads(t, 1, 3)
	if len(byHeight) < 3 {
		t.Skip("need field blocks 1-3 in testdata")
	}
	dir := t.TempDir()
	rs, err := store.OpenRawBlockStoreWithOpts(dir, store.BlockStorageOpts{Layout: store.BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	blocks := [][]byte{gen, byHeight[1], byHeight[2], byHeight[3]}
	for _, raw := range blocks {
		if err := rs.Put(pow.BlockHashLE(raw[:80]), raw); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(rs.Dir(), "blk00000.dat")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const blockRecordHeaderLen = 44
	const blockRecordMagic = uint32(0x0CB16ED0)
	if binary.LittleEndian.Uint32(data[0:]) != blockRecordMagic {
		t.Fatal("missing record magic")
	}
	storedLen := binary.LittleEndian.Uint32(data[8:12])
	firstRec := blockRecordHeaderLen + int(storedLen)
	if firstRec >= len(data) {
		t.Fatalf("firstRec=%d file=%d", firstRec, len(data))
	}
	if err := os.WriteFile(path, data[:firstRec+8], 0o600); err != nil {
		t.Fatal(err)
	}
	tip, err := rs.ProbeBundledContiguousTip()
	if err != nil || tip != 0 {
		t.Fatalf("tip=%d err=%v want 0 after torn tail", tip, err)
	}
	got, err := rs.GetByContiguousHeight(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, gen) {
		t.Fatal("genesis contiguous read mismatch after truncate")
	}
}

func TestMainnetFieldSparseBlocksBundledHashGet(t *testing.T) {
	gen := mustMainnetGenesis(t)
	raw10006, ok := mainnetFieldBlockPayloads(t, 10006, 10006)[10006]
	if !ok {
		t.Fatal("missing field block 10006")
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), gen[:80])
	if err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStoreWithOpts(dir, store.BlockStorageOpts{Layout: store.BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Put(pow.BlockHashLE(gen[:80]), gen); err != nil {
		t.Fatal(err)
	}
	if err := rs.Put(pow.BlockHashLE(raw10006[:80]), raw10006); err != nil {
		t.Fatal(err)
	}
	appendTip, err := rs.ProbeBundledContiguousTip()
	if err != nil || appendTip != 1 {
		t.Fatalf("append-order tip=%d err=%v want 1", appendTip, err)
	}
	measured := store.MeasureContiguousBodiesOnDisk(j, rs, chain.MainnetDogecoin, 0, 0)
	if measured != 0 {
		t.Fatalf("chain contiguous=%d want 0 without headers 1..10006", measured)
	}
	got, err := rs.Get(pow.BlockHashLE(raw10006[:80]))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw10006) {
		t.Fatal("hash get mismatch for sparse block")
	}
}

func TestMainnetFieldBlock10006BundledPutGet(t *testing.T) {
	var spec mainnetCanonicalBlockSpec
	for _, s := range mainnetCanonicalBlockSpecs {
		if s.Height == 10006 {
			spec = s
			break
		}
	}
	if spec.Height != 10006 {
		t.Fatal("missing height 10006 spec")
	}
	raw, err := buildMainnetCanonicalBlockRaw(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 213 {
		t.Fatalf("len=%d want 213", len(raw))
	}
	min := store.MinRawBlockBytes(chain.MainnetDogecoin, spec.Height)
	if min > len(raw) {
		t.Fatalf("min=%d blocks 213 B real block at 10006", min)
	}
	dir := t.TempDir()
	rs, err := store.OpenRawBlockStoreWithOpts(dir, store.BlockStorageOpts{Layout: store.BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	id := pow.BlockHashLE(raw[:80])
	if err := rs.Put(id, raw); err != nil {
		t.Fatal(err)
	}
	got, err := rs.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatal("get mismatch")
	}
	if pow.BlockHashHex(got[:80]) != spec.WantHash {
		t.Fatalf("hash %s want %s", pow.BlockHashHex(got[:80]), spec.WantHash)
	}
}
