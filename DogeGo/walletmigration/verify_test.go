// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package walletmigration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dogego/wallet/corewallet"
)

func TestDefaultOfflineSuitesNonEmpty(t *testing.T) {
	s := DefaultOfflineSuites()
	if len(s) != 4 {
		t.Fatalf("suites=%d", len(s))
	}
}

func TestSuiteCommandLineShape(t *testing.T) {
	for _, s := range DefaultOfflineSuites() {
		line := SuiteCommandLine(s)
		if !strings.HasPrefix(line, "go test ") {
			t.Fatalf("suite %q line=%q", s.Name, line)
		}
	}
}

func TestProbeFileNotBDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	live, err := ProbeFile(path, "", "mainnet")
	if err != nil {
		t.Fatal(err)
	}
	if live.Probe == nil || live.Probe.IsBDB {
		t.Fatalf("probe %#v", live.Probe)
	}
}

func TestProbeFileEmptyPath(t *testing.T) {
	_, err := ProbeFile("", "", "mainnet")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProbeFileSyntheticFixture(t *testing.T) {
	pub := append([]byte{0x03}, make([]byte, 32)...)
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 1)
	}
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestWalletDat(path, pub, secret); err != nil {
		t.Fatal(err)
	}
	live, err := ProbeFile(path, "", "mainnet")
	if err != nil {
		t.Fatal(err)
	}
	if live.Probe == nil || !live.Probe.CanImport || live.Probe.KeyCount != 1 {
		t.Fatalf("live %#v", live)
	}
	if !live.ExtractOK || live.ExtractedKeys != 1 {
		t.Fatalf("extract live=%#v", live)
	}
}

func TestProbeFileEncryptedDescriptorFixture(t *testing.T) {
	pub := append([]byte{0x02}, make([]byte, 32)...)
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 10)
	}
	passphrase := "descriptor-fixture"
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestEncryptedDescriptorWalletDat(path, pub, secret, passphrase); err != nil {
		t.Fatal(err)
	}
	live, err := ProbeFile(path, "", "mainnet")
	if err != nil {
		t.Fatal(err)
	}
	if live.Probe == nil || !live.Probe.NeedsPassphrase || live.Probe.EncryptedKeys != 1 {
		t.Fatalf("probe live=%#v", live)
	}
	live, err = ProbeFile(path, passphrase, "mainnet")
	if err != nil {
		t.Fatal(err)
	}
	if !live.ExtractOK || live.ExtractedKeys != 1 {
		t.Fatalf("extract live=%#v", live)
	}
}

func TestProbeFileMultiPoolFixture(t *testing.T) {
	pub := append([]byte{0x03}, make([]byte, 32)...)
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 3)
	}
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestWalletDatWithPools(path, pub, secret, []int64{4, 8}); err != nil {
		t.Fatal(err)
	}
	live, err := ProbeFile(path, "", "testnet")
	if err != nil {
		t.Fatal(err)
	}
	if live.Probe == nil || live.Probe.PoolCount != 2 || live.Probe.PoolPubkeys != 2 {
		t.Fatalf("live probe %#v", live.Probe)
	}
	if live.Probe.PoolKeysMatched != 2 || live.Probe.PoolKeysUnmatched != 0 {
		t.Fatalf("matched=%d unmatched=%d", live.Probe.PoolKeysMatched, live.Probe.PoolKeysUnmatched)
	}
	if live.Probe.PoolIndexMin == nil || *live.Probe.PoolIndexMin != 4 || live.Probe.PoolIndexMax == nil || *live.Probe.PoolIndexMax != 8 {
		t.Fatalf("indices %#v", live.Probe)
	}
}

func TestProbeFileMixedPoolFixture(t *testing.T) {
	spendPub := append([]byte{0x03}, make([]byte, 32)...)
	poolOnlyPub := append([]byte{0x02}, make([]byte, 32)...)
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 5)
	}
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestWalletDatWithMixedPool(path, spendPub, secret, poolOnlyPub, 3, 9); err != nil {
		t.Fatal(err)
	}
	live, err := ProbeFile(path, "", "mainnet")
	if err != nil {
		t.Fatal(err)
	}
	if live.Probe == nil || live.Probe.PoolCount != 2 || live.Probe.PoolPubkeys != 2 {
		t.Fatalf("probe %#v", live.Probe)
	}
	if live.Probe.PoolKeysMatched != 1 || live.Probe.PoolKeysUnmatched != 1 {
		t.Fatalf("matched=%d unmatched=%d", live.Probe.PoolKeysMatched, live.Probe.PoolKeysUnmatched)
	}
}
