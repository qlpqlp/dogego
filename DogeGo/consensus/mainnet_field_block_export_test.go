//go:build field_evidence

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

// TestExportMainnetFieldBlockHex logs mainnet block hex from local dogedata for core_block_vectors.json.
// Run: go test ./consensus -run TestExportMainnetFieldBlockHex -v
func TestExportMainnetFieldBlockHex(t *testing.T) {
	chainDir := filepath.Join("..", "dogedata", "mainnet")
	gen, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderChain(chainDir, gen[:80])
	if err != nil {
		t.Skip("no local mainnet headers:", err)
	}
	rawStore, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := rawStore.Count()
	t.Logf("stored_blocks=%d", n)
	for h := int64(1); h <= 5; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			t.Fatalf("header %d: %v", h, err)
		}
		id := pow.BlockHashLE(h80)
		raw, err := rawStore.Get(id)
		if err != nil {
			t.Logf("HEIGHT_%d missing body: %v", h, err)
			continue
		}
		t.Logf("HEIGHT_%d_HASH=%s", h, pow.BlockHashHex(h80))
		t.Logf("HEIGHT_%d_HEX=%s", h, strings.ToUpper(hex.EncodeToString(raw)))
	}
}
