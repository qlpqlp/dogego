//go:build datadir_diag

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/consensus"
	"dogego/pow"
	"dogego/store"
)

func openDatadirBlockStore(t *testing.T, chainDir string, utxo *store.UtxoCache, p chain.Params) *BlockStoreCtx {
	t.Helper()
	gen, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderChain(chainDir, gen[:80])
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
	bs := NewBlockStoreCtx(j, nil, p, raw, txIx, utxo)
	if cp, err := store.LoadRawBlockSyncCheckpoint(chainDir); err == nil && cp.ContiguousRawHeight >= 0 {
		bs.TrySeedContiguousFromCheckpoint(cp.ContiguousRawHeight)
	}
	bs.RefreshContiguousTip()
	return bs
}

// Run: go test -tags datadir_diag ./node -run TestDatadirConnectFromSnapshot -v -timeout 10m
func TestDatadirConnectFromSnapshot(t *testing.T) {
	chainDir := filepath.Join("..", "dogedata", "mainnet")
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	utxo, err := store.LoadUtxoSnapshot(store.UtxoSnapshotPath(chainDir))
	if err != nil {
		t.Fatal(err)
	}
	if utxo == nil {
		t.Fatal("no utxo.cache - stop node and let it save, or replay with TestDatadirConnectAt6857")
	}
	t.Logf("utxo.cache tip=%d outputs=%d", utxo.TipHeight(), utxo.Count())
	bs := openDatadirBlockStore(t, chainDir, utxo, p)
	next := utxo.TipHeight() + 1
	h80, err := bs.Journal.ReadHeaderAt(next)
	if err != nil {
		t.Fatal(err)
	}
	rawBytes, err := bs.Raw.Get(pow.BlockHashLE(h80))
	if err != nil {
		t.Fatal(err)
	}
	if err := bs.ConnectBlockAtPayload(rawBytes, next); err != nil {
		t.Fatalf("ConnectBlockAtPayload height %d: %v%s", next, err, consensus.LegacySubsidyBugHint(err))
	}
	t.Logf("connected height %d ok (outputs=%d)", next, utxo.Count())
}

// Run: go test -tags datadir_diag ./node -run TestDatadirConnectBurst -v -timeout 15m
func TestDatadirConnectBurst(t *testing.T) {
	chainDir := filepath.Join("..", "dogedata", "mainnet")
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	utxo, err := store.LoadUtxoSnapshot(store.UtxoSnapshotPath(chainDir))
	if err != nil {
		t.Fatal(err)
	}
	if utxo == nil {
		t.Skip("no utxo.cache")
	}
	bs := openDatadirBlockStore(t, chainDir, utxo, p)
	initial := utxo.TipHeight()
	const maxSteps = 64
	for step := 0; step < maxSteps; step++ {
		prev := utxo.TipHeight()
		if err := bs.SyncUtxoCacheBounded(128); err != nil {
			t.Fatalf("SyncUtxoCacheBounded at tip %d: %v%s", prev, err, consensus.LegacySubsidyBugHint(err))
		}
		if utxo.TipHeight() == prev {
			break
		}
	}
	t.Logf("burst connect: %d -> %d contiguous=%d", initial, utxo.TipHeight(), bs.ContiguousRawHeight())
}
