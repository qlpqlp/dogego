// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
)

// Active-write crash tests (Milestone B): simulate force-kill at each atomic-write phase
// and verify reopen + purge converges without manual repair.

func testGenesisRawBlock(t *testing.T, net chain.Network) ([]byte, [32]byte, []byte) {
	t.Helper()
	p, err := chain.ParamsFor(net)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	raw := MakeTestBlockRaw(t, g80[:])
	id := pow.BlockHashLE(raw[:80])
	return g80[:], id, raw
}

func TestCrashActiveRawPut_CompleteTmpOnly(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, id, genesisRaw := testGenesisRawBlock(t, chain.RebootTestnet)
	path := filepath.Join(raw.Dir(), hex.EncodeToString(id[:])+".bin")
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, genesisRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if raw.Has(id) {
		t.Fatal("complete .tmp must not count as stored block")
	}
	n, err := raw.PurgeStaleRawBlockTemps()
	if err != nil || n != 1 {
		t.Fatalf("purge tmp: n=%d err=%v", n, err)
	}
	if err := raw.Put(id, genesisRaw); err != nil {
		t.Fatal(err)
	}
	if !raw.Has(id) {
		t.Fatal("expected block after re-Put")
	}
}

func TestCrashKillBeforeRawPutRename(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, id, genesisRaw := testGenesisRawBlock(t, chain.RebootTestnet)
	SetAbortBeforeRawPutRenameForTest(true)
	putErr := raw.Put(id, genesisRaw)
	if putErr == nil || putErr != errAbortBeforeRawPutRename {
		t.Fatalf("Put err=%v want %v", putErr, errAbortBeforeRawPutRename)
	}
	tmpPath := filepath.Join(raw.Dir(), hex.EncodeToString(id[:])+".bin.tmp")
	if _, err := os.Stat(tmpPath); err != nil {
		t.Fatal("expected .tmp after simulated kill")
	}
	if raw.Has(id) {
		t.Fatal("must not have final .bin before recovery")
	}
	if _, err := raw.PurgeStaleRawBlockTemps(); err != nil {
		t.Fatal(err)
	}
	if err := raw.Put(id, genesisRaw); err != nil {
		t.Fatal(err)
	}
	if !raw.Has(id) {
		t.Fatal("expected stored block after recovery Put")
	}
}

func TestCrashActiveRawPut_PartialTmp(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, id, genesisRaw := testGenesisRawBlock(t, chain.RebootTestnet)
	tmpPath := filepath.Join(raw.Dir(), hex.EncodeToString(id[:])+".bin.tmp")
	if err := os.WriteFile(tmpPath, genesisRaw[:120], 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.PurgeStaleRawBlockTemps(); err != nil {
		t.Fatal(err)
	}
	if raw.Has(id) {
		t.Fatal("partial .tmp must not leave a valid block")
	}
	if err := raw.Put(id, genesisRaw); err != nil {
		t.Fatal(err)
	}
}

func TestCrashActiveRawPut_PartialBinAfterRename(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := MakeTestBlockRaw(t, g80[:])
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	id := pow.BlockHashLE(genesisRaw[:80])
	stub := make([]byte, MainnetGenesisStubTestBytes)
	copy(stub[:80], genesisRaw[:80])
	path := filepath.Join(raw.Dir(), hex.EncodeToString(id[:])+".bin")
	if err := os.WriteFile(path, stub, 0o600); err != nil {
		t.Fatal(err)
	}
	n, _, err := PurgeInadequateRawBodies(j, raw, chain.MainnetDogecoin)
	if err != nil || n != 1 {
		t.Fatalf("purge inadequate: n=%d err=%v", n, err)
	}
	if raw.Has(id) {
		t.Fatal("undersized .bin should be removed")
	}
	full := MakeTestBlockRaw(t, g80[:])
	if err := raw.Put(id, full); err != nil {
		t.Fatal(err)
	}
}

func TestCrashActiveHeaderSegment_CompleteTmpDiscarded(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, err := OpenHeaderChain(chainDir, gen[:])
	if err != nil {
		t.Fatal(err)
	}
	prev := append([]byte(nil), gen[:]...)
	for h := int64(1); h <= 3; h++ {
		h80 := append([]byte(nil), prev...)
		ph := pow.BlockHashLE(prev)
		copy(h80[4:36], ph[:])
		h80[76] ^= byte(h)
		if err := j.AppendHeaders([][]byte{h80}); err != nil {
			t.Fatal(err)
		}
		prev = append([]byte(nil), h80...)
	}
	tip, err := j.TipHeight()
	if err != nil || tip != 3 {
		t.Fatalf("tip=%d err=%v", tip, err)
	}
	segPath := filepath.Join(chainDir, "headers", "seg", "0000000000.bin")
	data, err := os.ReadFile(segPath)
	if err != nil {
		t.Fatal(err)
	}
	// Kill after WriteFile(tmp) but before Rename: full next header only in .tmp.
	next := append([]byte(nil), prev...)
	ph := pow.BlockHashLE(prev)
	copy(next[4:36], ph[:])
	next[76] ^= 4
	if err := os.WriteFile(segPath+".tmp", append(data, next...), 0o600); err != nil {
		t.Fatal(err)
	}
	j2, err := OpenHeaderChain(chainDir, gen[:])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(segPath + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("expected segment .tmp purged on open")
	}
	postTip, _ := j2.TipHeight()
	if postTip != tip {
		t.Fatalf("uncommitted segment .tmp must not advance tip: got %d want %d", postTip, tip)
	}
}

func TestCrashActiveHeaderManifestTmpPurged(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, err := OpenHeaderChain(chainDir, gen[:])
	if err != nil {
		t.Fatal(err)
	}
	tip, _ := j.TipHeight()
	mp := headerManifestPath(chainDir)
	if err := os.WriteFile(mp+".tmp", []byte(`{"version":1,"tip_height":999}`), 0o600); err != nil {
		t.Fatal(err)
	}
	j2, err := OpenHeaderChain(chainDir, gen[:])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mp + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("expected manifest .tmp purged on open")
	}
	postTip, _ := j2.TipHeight()
	if postTip != tip {
		t.Fatalf("manifest .tmp must not change tip: got %d want %d", postTip, tip)
	}
}
