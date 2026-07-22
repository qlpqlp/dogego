// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/rpc"
	"dogego/store"
)

// assertGenesisStubReplaced checks that an undersized genesis stub was purged; sweep may then
// store the real chainparams genesis via EnsureLocalGenesis (raw.Has stays true with adequate bytes).
func assertGenesisStubReplaced(t *testing.T, bs *BlockStoreCtx, genHash [32]byte) {
	t.Helper()
	if bs == nil || bs.Raw == nil {
		t.Fatal("nil block store")
	}
	if !bs.Raw.Has(genHash) {
		if NeedsGenesisBlock(bs) {
			t.Fatal("genesis missing after stub purge (expected local genesis restore)")
		}
		return
	}
	body, err := bs.Raw.Get(genHash)
	if err != nil {
		t.Fatal(err)
	}
	min := store.MinRawBlockBytes(bs.chainNet(), 0)
	if len(body) < min {
		t.Fatalf("genesis raw still undersized (%d bytes, need >= %d)", len(body), min)
	}
}

func TestAutoRecoverGenesisSanityAcceptsMatchingGenesis(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(t.TempDir()+"/headers.bin", gen[:])
	if err != nil {
		t.Fatal(err)
	}
	if err := autoRecoverGenesisSanity(j, p); err != nil {
		t.Fatalf("expected matching genesis sanity pass, got %v", err)
	}
}

func TestAutoRecoverGenesisSanityRejectsMismatchedGenesis(t *testing.T) {
	mainnet, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	testnet, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	mainGenesis, err := pow.Header80FromParams(mainnet)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(t.TempDir()+"/headers.bin", mainGenesis[:])
	if err != nil {
		t.Fatal(err)
	}
	if err := autoRecoverGenesisSanity(j, testnet); err == nil {
		t.Fatal("expected genesis mismatch error")
	}
}

func TestAutoRecoverFilterRepairFnNilGuard(t *testing.T) {
	if fn := autoRecoverFilterRepairFn(nil, "", nil, nil, nil); fn != nil {
		t.Fatal("expected nil repair function when required dependencies are missing")
	}
}

func TestAutoRecoverSweepNoopWithNilDeps(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	rewound, err := autoRecoverSweep("", nil, nil, p, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error for noop sweep: %v", err)
	}
	if rewound {
		t.Fatal("expected no rewind when no journals are available")
	}
}

func TestAutoRecoverSweepReconcilesRawBlockSyncCheckpoint(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := store.SaveRawBlockSyncCheckpoint(dir, store.RawBlockSyncCheckpoint{NextProbeHeight: 900, ContiguousRawHeight: 850}); err != nil {
		t.Fatal(err)
	}
	bs := &BlockStoreCtx{}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 120
	bs.contiguousMu.Unlock()
	_, err = autoRecoverSweep(dir, nil, nil, p, bs, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadRawBlockSyncCheckpoint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.NextProbeHeight != 121 || got.ContiguousRawHeight != 120 {
		t.Fatalf("checkpoint probe=%d cont=%d want 121/120", got.NextProbeHeight, got.ContiguousRawHeight)
	}
}

func TestAutoRecoverSweepInvokesRepairFilters(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	rewound, err := autoRecoverSweep("", nil, nil, p, nil, func() { called = true })
	if err != nil {
		t.Fatalf("unexpected error for sweep with callback: %v", err)
	}
	if rewound {
		t.Fatal("expected no rewind when no journals are available")
	}
	if !called {
		t.Fatal("expected repairFilters callback to be called")
	}
}

func TestAutoRecoverPostRewindResetsSyncState(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(t.TempDir()+"/headers.bin", gen[:])
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, nil, nil, nil)
	bs.contiguousMu.Lock()
	bs.contiguousTip = 15
	bs.contiguousMu.Unlock()

	rawFill := &progressiveRawState{
		inFlight:         map[int64][32]byte{2: {}},
		inFlightLane:     map[int64]int{2: 1},
		laneAddr:         map[int]string{1: "127.0.0.1:22556"},
		laneDownloadSince: map[int]time.Time{1: time.Now()},
		idleFull:         true,
	}
	pending := false

	autoRecoverPostRewind(bs, rawFill, &pending)

	if !pending {
		t.Fatal("expected headerCatchUpPending to be set true")
	}
	if got := bs.ContiguousRawHeight(); got != -1 {
		t.Fatalf("expected contiguous tip reset to -1, got %d", got)
	}
	if len(rawFill.inFlight) != 0 || len(rawFill.inFlightLane) != 0 {
		t.Fatal("expected in-flight claims to be cleared")
	}
	if len(rawFill.laneAddr) != 0 || len(rawFill.laneDownloadSince) != 0 {
		t.Fatal("expected lane sync metadata to be cleared")
	}
	if rawFill.idleFull {
		t.Fatal("expected raw sync to be re-armed after rewind")
	}
}

func TestAutoRecoverHeadersSkipsWhenRelaxedPoW(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	p.RelaxedPoW = true
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(t.TempDir()+"/headers.bin", gen[:])
	if err != nil {
		t.Fatal(err)
	}
	rewound, err := autoRecoverHeaders(j, nil, p, nil)
	if err != nil || rewound {
		t.Fatalf("RelaxedPoW chains skip stored validation in sweep: rewound=%v err=%v", rewound, err)
	}
}

func TestAutoRecoverHeadersValidatesRebootTestnetPoW(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	if p.RelaxedPoW {
		t.Fatal("expected real PoW on reboot testnet")
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(t.TempDir()+"/headers.bin", gen[:])
	if err != nil {
		t.Fatal(err)
	}
	bad := make([]byte, 80)
	genHash := pow.BlockHashLE(gen[:])
	copy(bad[0:4], gen[0:4])
	copy(bad[4:36], genHash[:])
	binaryLETime(bad, 1_700_000_060)
	if err := j.AppendHeaders([][]byte{bad}); err != nil {
		t.Fatal(err)
	}
	rewound, err := autoRecoverHeaders(j, nil, p, nil)
	if err == nil && !rewound {
		t.Fatal("expected header validation failure on invalid PoW header")
	}
}

func TestAutoRecoverHeadersReturnsErrorOnCorruptJournal(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(t.TempDir()+"/headers.bin", gen[:])
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt chain fixture: valid prev link but impossible nBits at height 1 on mainnet.
	bad := make([]byte, 80)
	copy(bad, gen[:])
	genHash := pow.BlockHashLE(gen[:])
	copy(bad[4:36], genHash[:])
	binaryLETime(bad, 1_700_000_000)
	bad[72] = 0xff
	bad[73] = 0xff
	bad[74] = 0xff
	bad[75] = 0xff
	if err := j.AppendHeaders([][]byte{bad}); err != nil {
		t.Fatal(err)
	}

	rewound, err := autoRecoverHeaders(j, nil, p, nil)
	if err == nil {
		t.Fatal("expected corrupt journal validation error (bad nBits)")
	}
	if rewound {
		t.Fatal("did not expect rewind for non-recoverable bad-prev corruption class")
	}
}

func TestAutoRecoverSweepContinuesAfterHeaderValidationFailure(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(t.TempDir()+"/headers.bin", gen[:])
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt chain fixture: valid prev link but impossible nBits at height 1 on mainnet.
	bad := make([]byte, 80)
	copy(bad, gen[:])
	genHash := pow.BlockHashLE(gen[:])
	copy(bad[4:36], genHash[:])
	binaryLETime(bad, 1_700_000_000)
	bad[72] = 0xff
	bad[73] = 0xff
	bad[74] = 0xff
	bad[75] = 0xff
	if err := j.AppendHeaders([][]byte{bad}); err != nil {
		t.Fatal(err)
	}

	called := false
	rewound, sweepErr := autoRecoverSweep("", j, nil, p, nil, func() { called = true })
	if sweepErr != nil {
		t.Fatalf("expected sweep to continue despite header validation failure, got %v", sweepErr)
	}
	if rewound {
		t.Fatal("did not expect rewind for non-recoverable bad-nBits corruption class")
	}
	if !called {
		t.Fatal("expected sweep to continue and call repair callback")
	}
}

func TestRunLocalHeaderJournalRecoveryRewindsOnBadNBits(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(t.TempDir()+"/headers.bin", gen[:])
	if err != nil {
		t.Fatal(err)
	}
	for h := int64(1); h <= 32; h++ {
		h80 := make([]byte, 80)
		copy(h80, gen[:])
		binaryLETime(h80, 1_700_000_000+uint32(h)*60)
		if err := j.AppendHeaders([][]byte{h80}); err != nil {
			t.Fatal(err)
		}
	}
	bs := NewBlockStoreCtx(j, nil, p, nil, nil, nil)
	rewound, rerr := runLocalHeaderJournalRecovery(j, nil, p, bs, fmt.Errorf("header batch index 0 (chain height 1): bad nBits want 0x1d00ffff got 0x1d00ba8a"))
	if !rewound {
		t.Fatal("expected local header recovery rewind on bad nBits class")
	}
	if rerr != nil && !IsHeaderRewindRetryErr(rerr) {
		t.Fatalf("unexpected rewind error shape: %v", rerr)
	}
	if tip, _ := j.TipHeight(); tip >= 32 {
		t.Fatalf("expected rewind to lower tip, got %d", tip)
	}
}

func TestRunLocalHeaderJournalRecoveryRewindsOnCheckpointMismatch(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(t.TempDir()+"/headers.bin", gen[:])
	if err != nil {
		t.Fatal(err)
	}
	for h := int64(1); h <= 16; h++ {
		h80 := make([]byte, 80)
		copy(h80, gen[:])
		binaryLETime(h80, 1_700_100_000+uint32(h)*60)
		if err := j.AppendHeaders([][]byte{h80}); err != nil {
			t.Fatal(err)
		}
	}
	mismatchErr := fmt.Errorf("header batch index 0 (chain height 12): header at height 12: checkpoint hash mismatch (got abc want def)")
	rewound, rerr := runLocalHeaderJournalRecovery(j, nil, p, nil, mismatchErr)
	if !rewound {
		t.Fatal("expected checkpoint mismatch to trigger rewind")
	}
	if !IsHeaderRewindRetryErr(rerr) {
		t.Fatalf("expected retry-shaped rewind error, got %v", rerr)
	}
	tip, _ := j.TipHeight()
	if tip != 11 {
		t.Fatalf("expected checkpoint rewind to height 11, got %d", tip)
	}
}

func TestRunLocalHeaderJournalRecoveryRewindsOnAuxpowValidationErr(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(t.TempDir()+"/headers.bin", gen[:])
	if err != nil {
		t.Fatal(err)
	}
	for h := int64(1); h <= 32; h++ {
		h80 := make([]byte, 80)
		copy(h80, gen[:])
		binaryLETime(h80, 1_700_200_000+uint32(h)*60)
		if err := j.AppendHeaders([][]byte{h80}); err != nil {
			t.Fatal(err)
		}
	}
	rewound, rerr := runLocalHeaderJournalRecovery(j, nil, p, nil, fmt.Errorf("height 10 aux: malformed auxpow"))
	if !rewound {
		t.Fatal("expected auxpow validation error to trigger rewind")
	}
	if !IsHeaderRewindRetryErr(rerr) {
		t.Fatalf("expected retry-shaped rewind error, got %v", rerr)
	}
	tip, _ := j.TipHeight()
	if tip >= 32 {
		t.Fatalf("expected rewind to lower tip, got %d", tip)
	}
}

func TestAutoRecoverSweepPurgesUndersizedRawBodyStub(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(chainDir, "headers.bin"), gen[:])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, raw, nil, nil)
	genHash := pow.BlockHashLE(gen[:])
	stubPath := filepath.Join(raw.Dir(), hex.EncodeToString(genHash[:])+".bin")
	if err := os.WriteFile(stubPath, make([]byte, store.MainnetGenesisStubTestBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	if !raw.Has(genHash) {
		t.Fatal("expected stub file to exist before auto recovery")
	}

	rewound, sweepErr := autoRecoverSweep(chainDir, j, nil, p, bs, nil)
	if sweepErr != nil {
		t.Fatalf("unexpected sweep error: %v", sweepErr)
	}
	if rewound {
		t.Fatal("did not expect header rewind for raw-stub purge-only case")
	}
	assertGenesisStubReplaced(t, bs, genHash)
}

func TestAutoRecoverFilterRepairFnWiringWithRealStores(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(chainDir, "headers.bin"), gen[:])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	txIx, err := store.OpenTxIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	filterIx, err := store.OpenBlockFilterIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}

	fn := autoRecoverFilterRepairFn(j, chainDir, filterIx, txIx, raw)
	if fn == nil {
		t.Fatal("expected non-nil filter repair function with all dependencies wired")
	}
	// Empty stores should remain a fast no-op repair invocation.
	fn()
}

func TestAutoRecoverSweepWithChainDirRunsIndexHooksSafely(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(chainDir, "headers.bin"), gen[:])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, raw, nil, nil)

	rewound, sweepErr := autoRecoverSweep(chainDir, j, nil, p, bs, nil)
	if sweepErr != nil {
		t.Fatalf("unexpected sweep error with chainDir/index hooks: %v", sweepErr)
	}
	if rewound {
		t.Fatal("did not expect rewind on clean fixture")
	}
}

func TestAutoRecoverSweepRefreshesContiguousTipFromDisk(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(chainDir, "headers.bin"), gen[:])
	if err != nil {
		t.Fatal(err)
	}
	// Extend headers so height 1 exists (committed scrypt PoW fixture).
	h1, err := rpc.AppendRebootTestnetMinedFixture(j, 1)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	// Store valid genesis via Put (ensures contiguous starts at 0).
	genRaw := store.MakeTestBlockRaw(t, gen[:])
	if err := raw.Put(pow.BlockHashLE(genRaw[:80]), genRaw); err != nil {
		t.Fatal(err)
	}
	// Simulate an on-disk body at height 1 that was stored ahead of cached contiguous tip.
	h1Hash := pow.BlockHashLE(h1)
	h1Path := filepath.Join(raw.Dir(), hex.EncodeToString(h1Hash[:])+".bin")
	if err := os.WriteFile(h1Path, make([]byte, 200), 0o600); err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, raw, nil, nil)
	bs.SeedContiguousTip(0)
	if got := bs.ContiguousRawHeight(); got != 0 {
		t.Fatalf("expected seeded contiguous tip 0, got %d", got)
	}
	rewound, sweepErr := autoRecoverSweep(chainDir, j, nil, p, bs, nil)
	if sweepErr != nil {
		t.Fatalf("unexpected sweep error: %v", sweepErr)
	}
	if rewound {
		t.Fatal("did not expect header rewind while refreshing contiguous tip")
	}
	if got := bs.ContiguousRawHeight(); got != 1 {
		t.Fatalf("expected contiguous tip refreshed to 1, got %d", got)
	}
}

func TestAutoRecoverSweepToleratesCorruptIndexArtifacts(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(chainDir, "headers.bin"), gen[:])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	txIx, err := store.OpenTxIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	filterIx, err := store.OpenBlockFilterIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, raw, txIx, nil)

	// Corruption fixtures: malformed tx index and filter files should not crash recovery sweeps.
	if err := os.WriteFile(filepath.Join(txIx.RootDir(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), []byte{0x01, 0x02}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filterIx.Dir(), "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.dat"), []byte{0x03}, 0o600); err != nil {
		t.Fatal(err)
	}

	repair := autoRecoverFilterRepairFn(j, chainDir, filterIx, txIx, raw)
	rewound, sweepErr := autoRecoverSweep(chainDir, j, nil, p, bs, repair)
	if sweepErr != nil {
		t.Fatalf("unexpected sweep error with corrupt index artifacts: %v", sweepErr)
	}
	if rewound {
		t.Fatal("did not expect rewind on index-artifact-only fixture")
	}
}

func TestAutoRecoverSweepIsIdempotentOnCorruptionFixtures(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(chainDir, "headers.bin"), gen[:])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	txIx, err := store.OpenTxIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	filterIx, err := store.OpenBlockFilterIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, raw, txIx, nil)

	// Mixed fixture: undersized raw stub + malformed tx/filter artifacts.
	genHash := pow.BlockHashLE(gen[:])
	stubPath := filepath.Join(raw.Dir(), hex.EncodeToString(genHash[:])+".bin")
	if err := os.WriteFile(stubPath, make([]byte, store.MainnetGenesisStubTestBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(txIx.RootDir(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), []byte{0x01}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filterIx.Dir(), "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.dat"), []byte{0x02}, 0o600); err != nil {
		t.Fatal(err)
	}
	repair := autoRecoverFilterRepairFn(j, chainDir, filterIx, txIx, raw)

	for i := 0; i < 3; i++ {
		rewound, sweepErr := autoRecoverSweep(chainDir, j, nil, p, bs, repair)
		if sweepErr != nil {
			t.Fatalf("sweep iteration %d error: %v", i+1, sweepErr)
		}
		if rewound {
			t.Fatalf("sweep iteration %d unexpectedly rewound headers", i+1)
		}
	}
	assertGenesisStubReplaced(t, bs, genHash)
}

func TestRunLocalHeaderJournalRecoveryBadNBitsBoundedToGenesisReset(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(t.TempDir()+"/headers.bin", gen[:])
	if err != nil {
		t.Fatal(err)
	}
	fillTo := func(tip int64) {
		cur, _ := j.TipHeight()
		for h := cur + 1; h <= tip; h++ {
			h80 := make([]byte, 80)
			copy(h80, gen[:])
			binaryLETime(h80, 1_701_000_000+uint32(h)*60)
			if err := j.AppendHeaders([][]byte{h80}); err != nil {
				t.Fatal(err)
			}
		}
	}
	const targetTip int64 = 64
	fillTo(targetTip)
	bs := NewBlockStoreCtx(j, nil, p, nil, nil, nil)

	for i := 0; i < 3; i++ {
		fillTo(targetTip)
		rewound, rerr := runLocalHeaderJournalRecovery(j, nil, p, bs, fmt.Errorf("bad nBits"))
		if !rewound {
			t.Fatalf("attempt %d expected rewind", i+1)
		}
		if rerr != nil && !IsHeaderRewindRetryErr(rerr) {
			t.Fatalf("attempt %d unexpected rewind error shape: %v", i+1, rerr)
		}
	}
	tip, _ := j.TipHeight()
	if tip != 0 {
		t.Fatalf("expected bounded retries to converge to genesis reset, got tip %d", tip)
	}
}

func TestAutoRecoverSweepPeriodicRuntimeSimulation(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(chainDir, "headers.bin"), gen[:])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	txIx, err := store.OpenTxIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	filterIx, err := store.OpenBlockFilterIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, raw, txIx, nil)
	repair := autoRecoverFilterRepairFn(j, chainDir, filterIx, txIx, raw)
	genHash := pow.BlockHashLE(gen[:])

	makeRuntimeCorruption := func(seed byte) {
		stubPath := filepath.Join(raw.Dir(), hex.EncodeToString(genHash[:])+".bin")
		if err := os.WriteFile(stubPath, make([]byte, store.MainnetGenesisStubTestBytes), 0o600); err != nil {
			t.Fatal(err)
		}
		txName := fmt.Sprintf("%064x", uint64(seed+1))
		if err := os.WriteFile(filepath.Join(txIx.RootDir(), txName), []byte{seed}, 0o600); err != nil {
			t.Fatal(err)
		}
		filterName := fmt.Sprintf("%064x.dat", uint64(seed+101))
		if err := os.WriteFile(filepath.Join(filterIx.Dir(), filterName), []byte{seed + 1}, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 4; i++ {
		makeRuntimeCorruption(byte(i))
		rewound, sweepErr := autoRecoverSweep(chainDir, j, nil, p, bs, repair)
		if sweepErr != nil {
			t.Fatalf("runtime sweep %d error: %v", i+1, sweepErr)
		}
		if rewound {
			t.Fatalf("runtime sweep %d unexpectedly rewound headers", i+1)
		}
		assertGenesisStubReplaced(t, bs, genHash)
	}
}

func TestAutoRecoverSweepMiniSoakRandomizedCorruption(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(chainDir, "headers.bin"), gen[:])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	txIx, err := store.OpenTxIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	filterIx, err := store.OpenBlockFilterIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, raw, txIx, nil)
	repair := autoRecoverFilterRepairFn(j, chainDir, filterIx, txIx, raw)
	genHash := pow.BlockHashLE(gen[:])
	stubPath := filepath.Join(raw.Dir(), hex.EncodeToString(genHash[:])+".bin")
	rng := rand.New(rand.NewSource(7))

	for i := 0; i < 20; i++ {
		injectedRawStub := false
		if rng.Intn(2) == 0 {
			if err := os.WriteFile(stubPath, make([]byte, store.MainnetGenesisStubTestBytes), 0o600); err != nil {
				t.Fatal(err)
			}
			injectedRawStub = true
		}
		if rng.Intn(2) == 0 {
			txName := fmt.Sprintf("%064x", uint64(i+1))
			if err := os.WriteFile(filepath.Join(txIx.RootDir(), txName), []byte{byte(i)}, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if rng.Intn(2) == 0 {
			filterName := fmt.Sprintf("%064x.dat", uint64(i+1000))
			if err := os.WriteFile(filepath.Join(filterIx.Dir(), filterName), []byte{byte(i + 1)}, 0o600); err != nil {
				t.Fatal(err)
			}
		}

		rewound, sweepErr := autoRecoverSweep(chainDir, j, nil, p, bs, repair)
		if sweepErr != nil {
			t.Fatalf("mini-soak sweep %d error: %v", i+1, sweepErr)
		}
		if rewound {
			t.Fatalf("mini-soak sweep %d unexpectedly rewound headers", i+1)
		}
		if injectedRawStub {
			assertGenesisStubReplaced(t, bs, genHash)
		}
	}
}

func TestAutoRecoverHeadersJournalTailRepairAfterCrash(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(chainDir, "headers.bin"), gen[:])
	if err != nil {
		t.Fatal(err)
	}
	for h := int64(1); h <= 3; h++ {
		if _, err := rpc.AppendRebootTestnetMinedFixture(j, int(h)); err != nil {
			t.Fatal(err)
		}
	}
	preTip, err := j.TipHeight()
	if err != nil {
		t.Fatal(err)
	}

	// Simulate crash/torn tail: append a partial header record.
	f, err := os.OpenFile(j.Path(), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(make([]byte, 17)); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen triggers OpenHeaderJournal tail repair.
	j2, err := store.OpenHeaderJournal(j.Path(), gen[:])
	if err != nil {
		t.Fatal(err)
	}
	postTip, err := j2.TipHeight()
	if err != nil {
		t.Fatal(err)
	}
	if postTip != preTip {
		t.Fatalf("expected repaired journal to keep tip %d, got %d", preTip, postTip)
	}

	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j2, nil, p, raw, nil, nil)
	rewound, sweepErr := autoRecoverSweep(chainDir, j2, nil, p, bs, nil)
	if sweepErr != nil {
		t.Fatalf("expected clean sweep after tail repair, got %v", sweepErr)
	}
	if rewound {
		t.Fatal("did not expect rewind after successful tail repair")
	}
}

func TestAutoRecoverSweepConvergesWithCorruptIndexArtifactsAfterRestart(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	if _, err := store.OpenHeaderJournal(filepath.Join(chainDir, "headers.bin"), gen[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := store.OpenRawBlockStore(chainDir); err != nil {
		t.Fatal(err)
	}
	txIx, err := store.OpenTxIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	filterIx, err := store.OpenBlockFilterIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}

	// Persist malformed artifacts before restart simulation.
	if err := os.WriteFile(filepath.Join(txIx.RootDir(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), []byte{0x01, 0x02, 0x03}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filterIx.Dir(), "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.dat"), []byte{0x04, 0x05}, 0o600); err != nil {
		t.Fatal(err)
	}

	// Simulate restart by reopening all stores.
	j2, err := store.OpenHeaderJournal(filepath.Join(chainDir, "headers.bin"), gen[:])
	if err != nil {
		t.Fatal(err)
	}
	raw2, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	txIx2, err := store.OpenTxIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	filterIx2, err := store.OpenBlockFilterIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j2, nil, p, raw2, txIx2, nil)
	repair := autoRecoverFilterRepairFn(j2, chainDir, filterIx2, txIx2, raw2)

	for i := 0; i < 2; i++ {
		rewound, sweepErr := autoRecoverSweep(chainDir, j2, nil, p, bs, repair)
		if sweepErr != nil {
			t.Fatalf("restart sweep %d error: %v", i+1, sweepErr)
		}
		if rewound {
			t.Fatalf("restart sweep %d unexpectedly rewound headers", i+1)
		}
	}
}
