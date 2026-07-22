// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"os"
	"path/filepath"
	"testing"

	"dogego/consensus"
)

func TestPqCommitmentsDefaultWhenFieldMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	w, err := LoadOrCreate(path, 0x71)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.EnsurePQReady(); err != nil {
		t.Fatal(err)
	}
	_, _, err = w.NextPQCommitment()
	if err != nil {
		t.Fatal(err)
	}
	w2, err := LoadOrCreate(path, 0x71)
	if err != nil {
		t.Fatal(err)
	}
	if !w2.PqCommitmentsEnabled() {
		t.Fatal("expected pq commitments default on when field omitted from wallet.json")
	}
}

func TestEnsurePQReadyAndNextCommitment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	w, err := LoadOrCreate(path, 0x71)
	if err != nil {
		t.Fatal(err)
	}
	if !w.PqCommitmentsEnabled() {
		t.Fatal("expected pq commitments enabled by default")
	}
	if err := w.EnsurePQReady(); err != nil {
		t.Fatal(err)
	}
	tag, hex1, err := w.NextPQCommitment()
	if err != nil || tag != consensus.PQTagFalcon || len(hex1) != 64 {
		t.Fatalf("first commit: tag=%q hex=%q err=%v", tag, hex1, err)
	}
	_, hex2, err := w.NextPQCommitment()
	if err != nil || hex2 == hex1 {
		t.Fatalf("commits should differ: %q vs %q", hex1, hex2)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 32 {
		t.Fatal("expected persisted wallet with pq fields")
	}
}
