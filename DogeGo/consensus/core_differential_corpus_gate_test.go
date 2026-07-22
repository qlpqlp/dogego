// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/pow"
)

// TestCoreDifferentialCorpusGate asserts minimum corpus sizes for the Core parity regression net.
func TestCoreDifferentialCorpusGate(t *testing.T) {
	t.Run("blocks", func(t *testing.T) {
		vecs := loadCoreBlockVectors(t)
		if len(vecs) < 40 {
			t.Fatalf("block corpus too small: %d rows", len(vecs))
		}
		var maxTip int64
		for _, v := range vecs {
			if v.ChainTipHeight > maxTip {
				maxTip = v.ChainTipHeight
			}
		}
		if maxTip < 511 {
			t.Fatalf("block chain_tip_height max=%d want >=511 (512-block connect)", maxTip)
		}
	})
	t.Run("headers", func(t *testing.T) {
		vecs := loadCoreHeaderVectors(t)
		if len(vecs) < 80 {
			t.Fatalf("header corpus too small: %d rows", len(vecs))
		}
	})
	t.Run("script_templates", func(t *testing.T) {
		vecs := loadCoreScriptVectors(t)
		if len(vecs) < 100 {
			t.Fatalf("script template corpus too small: %d rows", len(vecs))
		}
	})
	t.Run("mempool", func(t *testing.T) {
		vecs, err := LoadMempoolDifferentialVectors()
		if err != nil {
			t.Fatal(err)
		}
		if len(vecs) < 50 {
			t.Fatalf("mempool corpus too small: %d rows", len(vecs))
		}
		seen := map[string]bool{}
		for _, v := range vecs {
			seen[v.Template] = true
		}
		if len(seen) < 50 {
			t.Fatalf("mempool templates too few: %d unique", len(seen))
		}
	})
	t.Run("mainnet_field_blocks_fixture", func(t *testing.T) {
		path := filepath.Join("testdata", "mainnet_field_blocks.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("missing mainnet_field_blocks.json: %v", err)
		}
		raw = stripUTF8BOM(raw)
		var entries []mainnetFieldBlockEntry
		if err := json.Unmarshal(raw, &entries); err != nil {
			t.Fatal(err)
		}
		if len(entries) < 1 {
			t.Fatalf("mainnet_field_blocks.json empty")
		}
		gen, err := chain.MainnetGenesisBlockRaw()
		if err != nil {
			t.Fatal(err)
		}
		genHex := strings.ToUpper(hex.EncodeToString(gen))
		var real int
		for _, e := range entries {
			if e.Height > 0 && strings.ToUpper(strings.TrimSpace(e.Hex)) != genHex {
				real++
			}
		}
		if real < 7 {
			t.Fatalf("mainnet_field_blocks.json real blocks=%d want >=7 (canonical heights 1,2,3,100,200,272,10006)", real)
		}
		for _, spec := range mainnetCanonicalBlockSpecs {
			found := false
			for _, e := range entries {
				if e.Height == spec.Height {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("mainnet_field_blocks.json missing canonical height %d", spec.Height)
			}
		}
		if _, err := mainnetFieldMultiTxBlock15504Entry(); err == nil {
			found15504 := false
			for _, e := range entries {
				if e.Height == mainnetFieldMultiTxBlockHeight {
					found15504 = true
					break
				}
			}
			if !found15504 {
				t.Fatalf("mainnet_field_blocks.json missing committed multi-tx height %d", mainnetFieldMultiTxBlockHeight)
			}
		}
	})
	t.Run("mainnet_field_headers_fixture", func(t *testing.T) {
		vecs := loadCoreHeaderVectors(t)
		var n int
		for _, v := range vecs {
			if v.Kind == "field_header" {
				n++
			}
		}
		if n < 11 {
			t.Fatalf("core_header_vectors field_header rows=%d want >=11 (canonical heights + 10000/100000/371337-371339)", n)
		}
		for _, spec := range mainnetCanonicalBlockSpecs {
			found := false
			for _, v := range vecs {
				if v.Kind == "field_header" && v.Height == spec.Height {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("core_header_vectors missing field_header for canonical height %d", spec.Height)
			}
		}
	})
	t.Run("mainnet_checkpoint_headers", func(t *testing.T) {
		vecs := loadCoreHeaderVectors(t)
		var withHex, accept int
		var genesisAccept bool
		for _, v := range vecs {
			if v.Kind != "checkpoint" || v.Network != "mainnet" {
				continue
			}
			if v.Height == 0 && v.WantAccept {
				genesisAccept = true
			}
			if strings.TrimSpace(v.HeaderHex) != "" {
				withHex++
				if v.WantAccept {
					accept++
				}
				want, ok := chain.CheckpointHashAt(chain.MainnetDogecoin, v.Height)
				if !ok {
					t.Fatalf("checkpoint height %d missing from chain.MainnetHeaderCheckpoints", v.Height)
				}
				h80, err := hex.DecodeString(strings.TrimSpace(v.HeaderHex))
				if err != nil || len(h80) != 80 {
					t.Fatalf("checkpoint %d header_hex: %v", v.Height, err)
				}
				got := strings.ToLower(pow.BlockHashHex(h80))
				want = strings.ToLower(strings.TrimPrefix(want, "0x"))
				if got != want {
					t.Fatalf("checkpoint %d hash %s want %s", v.Height, got, want)
				}
			}
		}
		if !genesisAccept {
			t.Fatal("mainnet checkpoint height 0 must want_accept")
		}
		if withHex < 3 {
			t.Fatalf("mainnet checkpoint rows with header_hex=%d want >=3 (datadir or committed field headers)", withHex)
		}
		if accept < 3 {
			t.Fatalf("mainnet checkpoint accept rows with header_hex=%d want >=3", accept)
		}
	})
	t.Run("mainnet_field_block_vectors", func(t *testing.T) {
		vecs := loadCoreBlockVectors(t)
		wantHeights := []int64{1, 2, 3, 100, 200, 272, 10006}
		seen := map[int64]bool{}
		for _, v := range vecs {
			if v.Network != "mainnet" || v.Kind != "check_block_payload" || v.Source != "hex" {
				continue
			}
			if v.Height > 0 && v.WantAccept {
				seen[v.Height] = true
			}
		}
		for _, h := range wantHeights {
			if !seen[h] {
				t.Fatalf("core_block_vectors missing mainnet field block height %d", h)
			}
		}
	})
	t.Run("mainnet_field_auxpow_fixture", func(t *testing.T) {
		entries, err := LoadMainnetFieldAuxpowEntries()
		if err != nil {
			t.Fatalf("missing mainnet_field_auxpow.json: %v", err)
		}
		if len(entries) < 3 {
			t.Fatalf("mainnet_field_auxpow.json rows=%d want >=3 (auxpow activation window from 371337)", len(entries))
		}
		var at371337 bool
		for _, e := range entries {
			if e.Height == 371337 {
				at371337 = true
			}
			if strings.TrimSpace(e.HeaderHex) == "" || strings.TrimSpace(e.AuxHex) == "" {
				t.Fatalf("height %d missing header_hex or aux_hex", e.Height)
			}
		}
		if !at371337 {
			t.Fatal("mainnet_field_auxpow.json missing height 371337")
		}
	})
}
