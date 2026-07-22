// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import "testing"

func TestEffectiveMine_rebootTestnetDefaultsOn(t *testing.T) {
	m := Merged{Network: "testnet", NodeMode: "full", Mine: false}
	if !EffectiveMine(m, false, false) {
		t.Fatal("expected auto mine on reboot testnet")
	}
}

func TestEffectiveMine_cliMineFalse(t *testing.T) {
	m := Merged{Network: "reboottestnet", NodeMode: "full"}
	if EffectiveMine(m, true, false) {
		t.Fatal("CLI -mine=false should disable")
	}
}

func TestEffectiveMine_mainnetOff(t *testing.T) {
	m := Merged{Network: "mainnet", NodeMode: "full", Mine: true}
	if EffectiveMine(m, false, false) {
		t.Fatal("mine flag ignored on mainnet background miner")
	}
}

func TestApplyTestnetAutoMine_wizard(t *testing.T) {
	f := File{Network: "testnet", NodeMode: "full"}
	ApplyTestnetAutoMine(&f)
	if !f.Mine {
		t.Fatal("wizard seed should enable mine on testnet")
	}
}
