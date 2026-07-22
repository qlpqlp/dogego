// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestBIP39Vector1(t *testing.T) {
	m := strings.Repeat("abandon ", 11) + "about"
	want := "5eb00bbddcf069084889a8ab9155568165f5c453ccb85e70811aaed6f6da5fc19a5ac40b389cd370d086206dec8aa6c43daea6690f20ad3d8d48b2d2ce9e38e4"
	seed, err := MnemonicToSeed(m, "")
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(seed) != want {
		t.Fatalf("seed %x want %s", seed, want)
	}
}

func TestBIP39TrezorVectorsEmptyPassphrase(t *testing.T) {
	cases := []struct {
		mnemonic string
		seed     string
	}{
		{
			"legal winner thank year wave sausage worth useful legal winner thank yellow",
			"878386efb78845b3355bd15ea4d39ef97d179cb712b77d5c12b6be415fffeffe5f377ba02bf3f8544ab800b955e51fbff09828f682052a20faa6addbbddfb096",
		},
		{
			"letter advice cage absurd amount doctor acoustic avoid letter advice cage above",
			"77d6be9708c8218738934f84bbbb78a2e048ca007746cb764f0673e4b1812d176bbb173e1a291f31cf633f1d0bad7d3cf071c30e98cd0688b5bcce65ecaceb36",
		},
	}
	for i, tc := range cases {
		if err := ValidateMnemonic(tc.mnemonic); err != nil {
			t.Fatalf("case %d validate: %v", i, err)
		}
		seed, err := MnemonicToSeed(tc.mnemonic, "")
		if err != nil {
			t.Fatalf("case %d seed: %v", i, err)
		}
		if hex.EncodeToString(seed) != tc.seed {
			t.Fatalf("case %d seed %x want %s", i, seed, tc.seed)
		}
	}
}

func TestBIP39PassphraseTREZOR(t *testing.T) {
	m := "letter advice cage absurd amount doctor acoustic avoid letter advice cage above"
	want := "d71de856f81a8acc65e6fc851a38d4d7ec216fd0796d0a6827a3ad6ed5511a30fa280f12eb2e47ed2ac03b5c462a0358d18d69fe4f985ec81778c1b370b652a8"
	seed, err := MnemonicToSeed(m, "TREZOR")
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(seed) != want {
		t.Fatalf("seed %x want %s", seed, want)
	}
}

func TestBIP39NormalizeMnemonic(t *testing.T) {
	m := "  ABANDON\t\nabandon  " + strings.Repeat("abandon ", 9) + "about "
	if got := NormalizeMnemonic(m); got != strings.Repeat("abandon ", 11)+"about" {
		t.Fatalf("normalize: %q", got)
	}
}

func TestBIP39InvalidChecksum(t *testing.T) {
	m := strings.Repeat("abandon ", 11) + "ability"
	if err := ValidateMnemonic(m); err == nil {
		t.Fatal("expected checksum error")
	}
}

func TestRestoreFromMnemonic(t *testing.T) {
	dir := t.TempDir()
	p := byte(0x71)
	w, err := LoadOrCreate(dir+"/wallet.json", p)
	if err != nil {
		t.Fatal(err)
	}
	m := strings.Repeat("abandon ", 11) + "about"
	if err := w.RestoreFromMnemonic(m, ""); err != nil {
		t.Fatal(err)
	}
	if !w.HDEnabled() {
		t.Fatal("expected HD")
	}
	addr := w.DefaultAddress()
	if addr == "" {
		t.Fatal("empty address")
	}
	w2, err := LoadOrCreate(dir+"/wallet.json", p)
	if err != nil {
		t.Fatal(err)
	}
	if w2.DefaultAddress() != addr {
		t.Fatalf("persisted addr %s vs %s", w2.DefaultAddress(), addr)
	}
}
