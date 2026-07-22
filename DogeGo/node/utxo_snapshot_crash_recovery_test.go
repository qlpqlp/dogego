// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/store"
)

func TestLoadUtxoSnapshotAtStartup_quarantinesCorrupt(t *testing.T) {
	dir := t.TempDir()
	snapPath := store.UtxoSnapshotPath(dir)
	if err := os.WriteFile(snapPath, []byte("not-a-utxo-cache"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapPath+".tmp", []byte("torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	params, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	got, quarantined, err := LoadUtxoSnapshotAtStartup(snapPath, dir, nil, nil, params.Net)
	if err != nil {
		t.Fatal(err)
	}
	if !quarantined {
		t.Fatal("expected quarantine")
	}
	if got == nil || got.TipHeight() != -1 {
		t.Fatalf("want empty cache tip -1, got tip=%v", got)
	}
	if _, err := os.Stat(snapPath); !os.IsNotExist(err) {
		t.Fatal("corrupt cache should be renamed away")
	}
	if _, err := os.Stat(snapPath + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("stale .tmp should be purged")
	}
	matches, _ := filepath.Glob(snapPath + ".stale*")
	if len(matches) == 0 {
		t.Fatal("expected quarantined .stale file")
	}
}

func TestAutoRecoverSweep_purgesUtxoTmp(t *testing.T) {
	dir := t.TempDir()
	tmp := store.UtxoSnapshotPath(dir) + ".tmp"
	if err := os.WriteFile(tmp, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	params, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := autoRecoverSweep(dir, nil, nil, params, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatal("expected utxo.cache.tmp purged by autoRecoverSweep")
	}
}
