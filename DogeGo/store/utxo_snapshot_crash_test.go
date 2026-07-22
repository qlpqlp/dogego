// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"os"
	"path/filepath"
	"testing"

	"dogego/wire"
)

func TestPurgeStaleUtxoSnapshotTemps(t *testing.T) {
	dir := t.TempDir()
	tmp := UtxoSnapshotPath(dir) + ".tmp"
	if err := os.WriteFile(tmp, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := PurgeStaleUtxoSnapshotTemps(dir)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatal("expected .tmp removed")
	}
	n, err = PurgeStaleUtxoSnapshotTemps(dir)
	if err != nil || n != 0 {
		t.Fatalf("second purge n=%d err=%v", n, err)
	}
}

func TestLoadUtxoSnapshot_badMagic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "utxo.cache")
	if err := os.WriteFile(path, []byte("XXXX garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadUtxoSnapshot(path)
	if err == nil {
		t.Fatal("expected bad magic error")
	}
}

func TestLoadUtxoSnapshot_truncated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "utxo.cache")
	// Magic + incomplete header
	if err := os.WriteFile(path, []byte("DGUT\x01"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadUtxoSnapshot(path)
	if err == nil {
		t.Fatal("expected truncated error")
	}
}

func TestLoadUtxoSnapshot_partialCoinTear(t *testing.T) {
	u := NewUtxoCache()
	gen := wire.ParsedBlock{Txs: []*wire.Tx{{Version: 1, Vin: []wire.TxIn{{}}, Vout: []wire.TxOut{{Value: 50e8, PkScript: []byte{0xaa, 0xbb}}}}}}
	if err := u.ApplyBlock(&gen, 0); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "utxo.cache")
	if err := u.SaveSnapshot(path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 20 {
		t.Fatal("snapshot too small")
	}
	// Tear mid-file after a valid header so Load fails on coin payload.
	if err := os.WriteFile(path, raw[:len(raw)/2], 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = LoadUtxoSnapshot(path)
	if err == nil {
		t.Fatal("expected partial tear error")
	}
}

func TestPurgeStaleUtxoSnapshotTemps_preservesGoodCache(t *testing.T) {
	u := NewUtxoCache()
	gen := wire.ParsedBlock{Txs: []*wire.Tx{{Version: 1, Vin: []wire.TxIn{{}}, Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}}}}}
	if err := u.ApplyBlock(&gen, 0); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := UtxoSnapshotPath(dir)
	if err := u.SaveSnapshot(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".tmp", []byte("junk"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PurgeStaleUtxoSnapshotTemps(dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadUtxoSnapshot(path)
	if err != nil || loaded == nil || loaded.TipHeight() != 0 {
		t.Fatalf("good cache damaged: loaded=%v err=%v", loaded, err)
	}
}
