// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func putBundledTestChain(t *testing.T, chainDir string, nBlocks int) (*store.HeaderJournal, *store.RawBlockStore) {
	t.Helper()
	genRaw, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(chainDir, "headers.bin"), genRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStoreWithOpts(chainDir, store.BlockStorageOpts{Layout: store.BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Put(pow.BlockHashLE(genRaw[:80]), genRaw); err != nil {
		t.Fatal(err)
	}
	prev := genRaw[:80]
	for i := 1; i < nBlocks; i++ {
		h80 := append([]byte(nil), prev...)
		ph := pow.BlockHashLE(h80)
		copy(h80[4:36], ph[:])
		h80[76] ^= byte(i)
		body := store.MakeTestBlockRaw(t, h80)
		if err := j.AppendHeaders([][]byte{h80}); err != nil {
			t.Fatal(err)
		}
		if err := rs.Put(pow.BlockHashLE(body[:80]), body); err != nil {
			t.Fatal(err)
		}
		prev = h80
	}
	return j, rs
}

func truncateBundledAfterFirstRecord(t *testing.T, rawDir string) {
	t.Helper()
	const blockRecordHeaderLen = 44
	path := filepath.Join(rawDir, "blk00000.dat")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	storedLen := binary.LittleEndian.Uint32(data[8:12])
	firstRec := blockRecordHeaderLen + int(storedLen)
	if err := os.WriteFile(path, data[:firstRec+8], 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMaybeClampBundledContiguousFromDiskAfterTornTail(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, rs := putBundledTestChain(t, chainDir, 4)
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	bs.SeedContiguousTip(3)
	truncateBundledAfterFirstRecord(t, rs.Dir())
	if got := bs.maybeClampBundledContiguousFromDisk(); got != 0 {
		t.Fatalf("clamp tip=%d want 0", got)
	}
	if got := bs.ContiguousRawHeight(); got != 0 {
		t.Fatalf("contiguous=%d want 0", got)
	}
}

func TestAutoRecoverSweepClampsBundledContiguousAfterTornTail(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, rs := putBundledTestChain(t, chainDir, 4)
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	bs.SeedContiguousTip(3)
	truncateBundledAfterFirstRecord(t, rs.Dir())
	if err := store.SaveRawBlockSyncCheckpoint(chainDir, store.RawBlockSyncCheckpoint{NextProbeHeight: 500, ContiguousRawHeight: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := autoRecoverSweep(chainDir, j, nil, p, bs, nil); err != nil {
		t.Fatal(err)
	}
	if got := bs.ContiguousRawHeight(); got != 0 {
		t.Fatalf("contiguous=%d want 0 after sweep", got)
	}
	cp, err := store.LoadRawBlockSyncCheckpoint(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	if cp.ContiguousRawHeight != 0 || cp.NextProbeHeight != 1 {
		t.Fatalf("checkpoint probe=%d cont=%d want 1/0", cp.NextProbeHeight, cp.ContiguousRawHeight)
	}
}
