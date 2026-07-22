// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package corewallet

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"dogego/chain"
)

func TestFixtureWalletDatProbeExtractImport(t *testing.T) {
	pub := append([]byte{0x03}, bytes.Repeat([]byte{0xab}, 32)...)
	secret := bytes.Repeat([]byte{0xcd}, 32)
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := WriteTestWalletDat(path, pub, secret); err != nil {
		t.Fatal(err)
	}
	p, err := ProbeWalletDat(path, 0x9e)
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsBDB || !p.CanImport || p.KeyCount != 1 || p.Encrypted {
		t.Fatalf("probe %#v", p)
	}
	ex, err := ExtractDumpLines(path, 0x9e)
	if err != nil || ex.KeyCount != 1 || len(ex.Lines) < 2 {
		t.Fatalf("extract keys=%d err=%v lines=%d", ex.KeyCount, err, len(ex.Lines))
	}
	wantWIF, err := chain.EncodeWIF(secret, 0x9e, true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range ex.Lines {
		if strings.Contains(line, wantWIF) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing wif %q in %#v", wantWIF, ex.Lines)
	}
}

func TestFixtureEncryptedWalletDatProbeExtract(t *testing.T) {
	pub := append([]byte{0x03}, bytes.Repeat([]byte{0xbb}, 32)...)
	secret := bytes.Repeat([]byte{0x44}, 32)
	passphrase := "s3cret"
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := WriteTestEncryptedWalletDat(path, pub, secret, passphrase); err != nil {
		t.Fatal(err)
	}
	p, err := ProbeWalletDat(path, 0x9e)
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsBDB || !p.NeedsPassphrase || !p.CanImport || p.EncryptedKeys != 1 {
		t.Fatalf("probe %#v", p)
	}
	ex, err := ExtractDumpLinesWithPassphrase(path, 0x9e, passphrase)
	if err != nil || ex.KeyCount != 1 {
		t.Fatalf("extract keys=%d err=%v", ex.KeyCount, err)
	}
}

func TestFixtureEncryptedDescriptorWalletDatProbeExtract(t *testing.T) {
	pub := append([]byte{0x02}, bytes.Repeat([]byte{0xcc}, 32)...)
	secret := bytes.Repeat([]byte{0x55}, 32)
	passphrase := "descriptor-s3cret"
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := WriteTestEncryptedDescriptorWalletDat(path, pub, secret, passphrase); err != nil {
		t.Fatal(err)
	}
	p, err := ProbeWalletDat(path, 0x9e)
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsBDB || !p.NeedsPassphrase || !p.CanImport || p.EncryptedKeys != 1 {
		t.Fatalf("probe %#v", p)
	}
	ex, err := ExtractDumpLinesWithPassphrase(path, 0x9e, passphrase)
	if err != nil || ex.KeyCount != 1 {
		t.Fatalf("extract keys=%d err=%v", ex.KeyCount, err)
	}
	wantWIF, err := chain.EncodeWIF(secret, 0x9e, true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range ex.Lines {
		if strings.Contains(line, wantWIF) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing wif %q in %#v", wantWIF, ex.Lines)
	}
}

func TestFixtureWalletDatProbePoolCount(t *testing.T) {
	pub := append([]byte{0x03}, bytes.Repeat([]byte{0xde}, 32)...)
	secret := bytes.Repeat([]byte{0xef}, 32)
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := WriteTestWalletDatWithPool(path, pub, secret, 3); err != nil {
		t.Fatal(err)
	}
	p, err := ProbeWalletDat(path, 0x9e)
	if err != nil {
		t.Fatal(err)
	}
	if p.PoolCount != 1 || p.KeyCount != 1 || !p.CanImport {
		t.Fatalf("probe %#v", p)
	}
	if p.PoolKeysMatched != 1 {
		t.Fatalf("pool_keys_matched=%d", p.PoolKeysMatched)
	}
	if p.PoolKeysUnmatched != 0 {
		t.Fatalf("pool_keys_unmatched=%d", p.PoolKeysUnmatched)
	}
	if p.Hint == "" || !strings.Contains(p.Hint, "keypool") {
		t.Fatalf("hint=%q", p.Hint)
	}
}

func TestFixtureWalletDatProbeMultiPoolIndices(t *testing.T) {
	pub := append([]byte{0x02}, bytes.Repeat([]byte{0xaa}, 32)...)
	secret := bytes.Repeat([]byte{0xbb}, 32)
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := WriteTestWalletDatWithPools(path, pub, secret, []int64{2, 7, 11}); err != nil {
		t.Fatal(err)
	}
	p, err := ProbeWalletDat(path, 0x9e)
	if err != nil {
		t.Fatal(err)
	}
	if p.PoolCount != 3 {
		t.Fatalf("pool_count=%d", p.PoolCount)
	}
	if p.PoolIndexMin == nil || *p.PoolIndexMin != 2 {
		t.Fatalf("min %#v", p.PoolIndexMin)
	}
	if p.PoolIndexMax == nil || *p.PoolIndexMax != 11 {
		t.Fatalf("max %#v", p.PoolIndexMax)
	}
	if p.PoolPubkeys != 3 {
		t.Fatalf("pool_pubkeys=%d", p.PoolPubkeys)
	}
	if p.PoolKeysMatched != 3 {
		t.Fatalf("pool_keys_matched=%d", p.PoolKeysMatched)
	}
	if p.PoolKeysUnmatched != 0 {
		t.Fatalf("pool_keys_unmatched=%d", p.PoolKeysUnmatched)
	}
	if len(p.PoolEntries) != 3 {
		t.Fatalf("pool_entries=%d", len(p.PoolEntries))
	}
	gotIdx := make(map[int64]bool)
	for _, e := range p.PoolEntries {
		if e.PubKeyHex == "" {
			t.Fatalf("missing pubkey hex at index %d", e.Index)
		}
		gotIdx[e.Index] = true
	}
	for _, want := range []int64{2, 7, 11} {
		if !gotIdx[want] {
			t.Fatalf("missing pool index %d in %#v", want, p.PoolEntries)
		}
	}
}

func TestFixtureWalletDatProbeMixedPool(t *testing.T) {
	spendPub := append([]byte{0x03}, bytes.Repeat([]byte{0xcc}, 32)...)
	poolOnlyPub := append([]byte{0x02}, bytes.Repeat([]byte{0xdd}, 32)...)
	secret := bytes.Repeat([]byte{0xee}, 32)
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := WriteTestWalletDatWithMixedPool(path, spendPub, secret, poolOnlyPub, 1, 5); err != nil {
		t.Fatal(err)
	}
	p, err := ProbeWalletDat(path, 0x9e)
	if err != nil {
		t.Fatal(err)
	}
	if p.PoolKeysMatched != 1 || p.PoolKeysUnmatched != 1 {
		t.Fatalf("matched=%d unmatched=%d", p.PoolKeysMatched, p.PoolKeysUnmatched)
	}
}
