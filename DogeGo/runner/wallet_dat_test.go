// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWalletDatPathExplicit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ResolveWalletDatPath(path); got != path {
		t.Fatalf("got=%q want=%q", got, path)
	}
}

func TestResolveWalletDatPathExplicitMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.dat")
	if got := ResolveWalletDatPath(missing); got != "" {
		t.Fatalf("got=%q want empty for missing explicit", got)
	}
}

func TestResolveWalletDatPathAutoEmpty(t *testing.T) {
	t.Setenv("DOGEGO_CORE_DATADIR", "")
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if got := ResolveWalletDatPath(""); got != "" {
		t.Fatalf("got=%q", got)
	}
}

func TestResolveWalletDatPathFromCoreDataDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.dat")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOGEGO_CORE_DATADIR", dir)
	if got := ResolveWalletDatPath(""); got != path {
		t.Fatalf("got=%q want=%q", got, path)
	}
}

func TestResolveWalletDatPathConfiguredExplicit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, configured := ResolveWalletDatPathConfigured(path)
	if got != path || !configured {
		t.Fatalf("got=%q configured=%v", got, configured)
	}
}

func TestResolveWalletDatPathConfiguredAuto(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.dat")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOGEGO_WALLET_DAT", "")
	t.Setenv("DOGEGO_CORE_DATADIR", dir)
	got, configured := ResolveWalletDatPathConfigured("")
	if got != "" || configured {
		t.Fatalf("configured lookup should not auto-discover: got=%q configured=%v", got, configured)
	}
	if auto := ResolveWalletDatPath(""); auto != path {
		t.Fatalf("auto=%q want=%q", auto, path)
	}
}

func TestResolveWalletDatPathConfiguredEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOGEGO_WALLET_DAT", path)
	got, configured := ResolveWalletDatPathConfigured("")
	if got != path || !configured {
		t.Fatalf("got=%q configured=%v", got, configured)
	}
}

func TestWalletDatImportEnabled(t *testing.T) {
	t.Setenv("DOGEGO_WALLET_DAT", "")
	if !WalletDatImportEnabled(true) {
		t.Fatal("require should enable")
	}
	if WalletDatImportEnabled(false) {
		t.Fatal("empty should not enable")
	}
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOGEGO_WALLET_DAT", path)
	if !WalletDatImportEnabled(false) {
		t.Fatal("configured path should enable")
	}
}

func TestWalletDatLiveImportNeeded(t *testing.T) {
	t.Setenv("DOGEGO_WALLET_DAT", "")
	if !WalletDatLiveImportNeeded(true) {
		t.Fatal("require")
	}
	if WalletDatLiveImportNeeded(false) {
		t.Fatal("empty env")
	}
	t.Setenv("DOGEGO_WALLET_DAT", filepath.Join(t.TempDir(), "wallet.dat"))
	if !WalletDatLiveImportNeeded(false) {
		t.Fatal("env set")
	}
}
