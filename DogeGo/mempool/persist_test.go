// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"os"
	"path/filepath"
	"testing"

	"dogego/wire"
)

func TestSaveLoadPersistedRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, PersistFileName)
	raw := [][]byte{{0x01, 0x00, 0x00, 0x01}, {0x02, 0x00, 0x00, 0x01}}
	if err := SavePersisted(path, raw, nil); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPersisted(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len %d", len(got))
	}
}

func TestSaveLoadPersistedFeeDeltasRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, PersistFileName)
	raw := [][]byte{minimalCoinbaseRaw(t)}
	deltas := map[string]int64{"abcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcd": 5000}
	if err := SavePersisted(path, raw, deltas); err != nil {
		t.Fatal(err)
	}
	snap, err := LoadPersistedSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Transactions) != 1 {
		t.Fatalf("txs %d", len(snap.Transactions))
	}
	if snap.FeeDeltas["abcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcd"] != 5000 {
		t.Fatalf("deltas %#v", snap.FeeDeltas)
	}
}

func TestLoadPersistedMissing(t *testing.T) {
	got, err := LoadPersisted(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("%v", got)
	}
}

func TestSavePersistedAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), PersistFileName)
	if err := SavePersisted(path, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".new"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind: %v", err)
	}
}

func TestRestoreFeeDeltasAfterLoad(t *testing.T) {
	p := New(10)
	raw := minimalCoinbaseRaw(t)
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	id := TxIDDisplayHex(tx.TxHash())
	deltas := map[string]int64{id: 9000}
	if err := p.Add(raw); err != nil {
		t.Fatal(err)
	}
	p.RestoreFeeDeltas(deltas)
	if got := p.FeeDeltaKoinu(id); got != 9000 {
		t.Fatalf("delta %d", got)
	}
}
