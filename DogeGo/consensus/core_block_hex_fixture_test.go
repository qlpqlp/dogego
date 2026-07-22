// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/store"
)

// TestCoreBlockGenesisHexFixtureValue logs the minimal genesis hex for core_block_vectors.json (source=hex).
func TestCoreBlockGenesisHexFixtureValue(t *testing.T) {
	raw, _ := store.TestMinimalBlock()
	t.Log("GENESIS_HEX=" + strings.ToUpper(hex.EncodeToString(raw)))
}

// TestMainnetGenesisHexFixtureValue logs mainnet chain_genesis hex for core_block_vectors.json (source=hex).
func TestMainnetGenesisHexFixtureValue(t *testing.T) {
	raw, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		t.Fatal(err)
	}
	t.Log("MAINNET_GENESIS_HEX=" + strings.ToUpper(hex.EncodeToString(raw)))
}

func TestCoreBlockOneHexFixtureValue(t *testing.T) {
	raw0, hash0 := minimalBlockRaw()
	_ = raw0
	raw1, _, err := minimalChainedBlockRaw(hash0, 1747000060, 166042)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("BLOCK_ONE_HEX=" + strings.ToUpper(hex.EncodeToString(raw1)))
}

func TestCoreBlockOneHexVectorMatchesMinimal(t *testing.T) {
	_, hash0 := minimalBlockRaw()
	raw1, _, err := minimalChainedBlockRaw(hash0, 1747000060, 166042)
	if err != nil {
		t.Fatal(err)
	}
	vecs := loadCoreBlockVectors(t)
	for _, v := range vecs {
		if v.Source != "hex" || v.Name != "core_hex_block_one_payload_accept" {
			continue
		}
		decoded, err := hex.DecodeString(strings.TrimSpace(v.Hex))
		if err != nil {
			t.Fatal(err)
		}
		if len(decoded) != len(raw1) {
			t.Fatalf("hex vector len=%d want %d", len(decoded), len(raw1))
		}
		for i := range raw1 {
			if decoded[i] != raw1[i] {
				t.Fatalf("hex mismatch at byte %d", i)
			}
		}
		return
	}
	t.Fatal("core_hex_block_one_payload_accept vector missing")
}

func TestCoreBlockHexVectorMatchesMinimal(t *testing.T) {
	raw, _ := store.TestMinimalBlock()
	vecs := loadCoreBlockVectors(t)
	for _, v := range vecs {
		if v.Source != "hex" || v.Name != "core_hex_genesis_payload_accept" {
			continue
		}
		decoded, err := hex.DecodeString(strings.TrimSpace(v.Hex))
		if err != nil {
			t.Fatal(err)
		}
		if len(decoded) != len(raw) {
			t.Fatalf("hex vector len=%d want %d", len(decoded), len(raw))
		}
		for i := range raw {
			if decoded[i] != raw[i] {
				t.Fatalf("hex mismatch at byte %d", i)
			}
		}
		return
	}
	t.Fatal("core_hex_genesis_payload_accept vector missing")
}

func TestMainnetGenesisHexVectorMatchesChain(t *testing.T) {
	raw, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		t.Fatal(err)
	}
	vecs := loadCoreBlockVectors(t)
	for _, v := range vecs {
		if v.Source != "hex" || v.Name != "mainnet_hex_genesis_payload_accept" {
			continue
		}
		decoded, err := hex.DecodeString(strings.TrimSpace(v.Hex))
		if err != nil {
			t.Fatal(err)
		}
		if len(decoded) != len(raw) {
			t.Fatalf("hex vector len=%d want %d", len(decoded), len(raw))
		}
		for i := range raw {
			if decoded[i] != raw[i] {
				t.Fatalf("hex mismatch at byte %d", i)
			}
		}
		return
	}
	t.Fatal("mainnet_hex_genesis_payload_accept vector missing")
}
