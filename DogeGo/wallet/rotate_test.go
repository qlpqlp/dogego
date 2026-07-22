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

	"dogego/chain"
)

func testWallet(t *testing.T) (*Disk, string) {
	t.Helper()
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	w, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	return w, path
}

func TestKeyRotationPrepareAndCancel(t *testing.T) {
	w, _ := testWallet(t)
	oldAddr := w.DefaultAddress()
	addr, err := w.BeginKeyRotation()
	if err != nil {
		t.Fatal(err)
	}
	if addr == oldAddr {
		t.Fatal("new rotation address should differ from current default")
	}
	st := w.RotationState()
	if !st.Active || st.NewAddress != addr {
		t.Fatalf("rotation state=%+v", st)
	}
	w.CancelKeyRotation()
	if w.RotationState().Active {
		t.Fatal("expected rotation cancelled")
	}
}

func TestKeyRotationFinalizeArchives(t *testing.T) {
	w, _ := testWallet(t)
	newAddr, err := w.BeginKeyRotation()
	if err != nil {
		t.Fatal(err)
	}
	archive, err := w.FinalizeKeyRotation()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("archive missing: %v", err)
	}
	if w.DefaultAddress() != newAddr {
		t.Fatalf("default=%s want %s", w.DefaultAddress(), newAddr)
	}
	if err := RemoveRotationArchive(archive); err != nil {
		t.Fatal(err)
	}
}
