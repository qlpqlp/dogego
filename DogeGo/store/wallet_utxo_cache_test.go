// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"path/filepath"
	"testing"
)

func TestWalletUtxoCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := WalletUtxoCachePath(dir)
	rows := []UtxoDumpRow{{TxID: "ab", Vout: 0, Value: 100, Height: 1}}
	key := WalletScriptsKey([][]byte{{0x76, 0xa9}})
	if err := SaveWalletUtxoCache(path, 42, key, rows); err != nil {
		t.Fatal(err)
	}
	got, ok := LoadWalletUtxoCache(path, 42, key)
	if !ok || len(got) != 1 || got[0].TxID != "ab" {
		t.Fatalf("load ok=%v got=%+v", ok, got)
	}
	_, ok = LoadWalletUtxoCache(path, 43, key)
	if ok {
		t.Fatal("expected miss on tip mismatch")
	}
}

func TestChainActiveManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	m := ChainActiveManifest{UtxoTipHeight: 100, UtxoTipBlockHash: "deadbeef", ContiguousRawHeight: 100}
	if err := SaveChainActiveManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	got, err := LoadChainActiveManifest(dir)
	if err != nil || got == nil || got.UtxoTipHeight != 100 {
		t.Fatalf("load err=%v got=%+v", err, got)
	}
	if filepath.Base(ChainActiveManifestPath(dir)) != "chain_active.manifest.json" {
		t.Fatal("unexpected manifest path")
	}
}
