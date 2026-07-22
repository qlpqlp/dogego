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
	"dogego/pow"
)

func TestPrepareChainDataDirMigrateLegacyTestnet(t *testing.T) {
	base := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	legacyHdr := filepath.Join(base, "headers.bin")
	if err := os.WriteFile(legacyHdr, g80[:], 0o600); err != nil {
		t.Fatal(err)
	}
	root, migrated, err := PrepareChainDataDir(base, chain.RebootTestnet, g80)
	if err != nil {
		t.Fatal(err)
	}
	if !migrated {
		t.Fatal("expected migration")
	}
	want := filepath.Join(base, "testnet")
	if root != want {
		t.Fatalf("chain root %q want %q", root, want)
	}
	if _, err := os.Stat(filepath.Join(want, "headers.bin")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacyHdr); !os.IsNotExist(err) {
		t.Fatalf("legacy file should be moved away")
	}
}

func TestPrepareChainDataDirLegacyGenesisMismatch(t *testing.T) {
	base := t.TempDir()
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	legacyHdr := filepath.Join(base, "headers.bin")
	wrong := append([]byte(nil), g80[:]...)
	wrong[0] ^= 0x01
	if err := os.WriteFile(legacyHdr, wrong, 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err = PrepareChainDataDir(base, chain.MainnetDogecoin, g80)
	if err == nil {
		t.Fatal("expected error for wrong legacy genesis")
	}
}

func TestPrepareChainDataDirNoLegacy(t *testing.T) {
	base := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	root, migrated, err := PrepareChainDataDir(base, chain.RebootTestnet, g80)
	if err != nil {
		t.Fatal(err)
	}
	if migrated {
		t.Fatal("unexpected migration")
	}
	if root != filepath.Join(base, "testnet") {
		t.Fatalf("root %q", root)
	}
}

func TestPrepareChainDataDirLegacyTestnetGenesisMainnetSelection(t *testing.T) {
	base := t.TempDir()
	pT, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gT, err := pow.Header80FromParams(pT)
	if err != nil {
		t.Fatal(err)
	}
	pM, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	gM, err := pow.Header80FromParams(pM)
	if err != nil {
		t.Fatal(err)
	}
	legacyHdr := filepath.Join(base, "headers.bin")
	if err := os.WriteFile(legacyHdr, gT[:], 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err = PrepareChainDataDir(base, chain.MainnetDogecoin, gM)
	if err == nil {
		t.Fatal("expected error when legacy file is testnet genesis but network is mainnet")
	}
}
