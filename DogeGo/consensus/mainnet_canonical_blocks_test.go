// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"strings"
	"testing"

	"dogego/chain"
	"dogego/pow"
)

func TestMainnetCanonicalBlockHashes(t *testing.T) {
	gen, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		t.Fatal(err)
	}
	prev := pow.BlockHashHex(gen[:80])
	if prev != "1a91e3dace36e2be3bf030a65679fe821aa1d6ef92e7c9902eb318182c355691" {
		t.Fatalf("genesis hash %s", prev)
	}
	for _, spec := range mainnetCanonicalBlockSpecs {
		raw, err := buildMainnetCanonicalBlockRaw(spec)
		if err != nil {
			t.Fatalf("height %d: %v", spec.Height, err)
		}
		if len(raw) < 160 {
			t.Fatalf("height %d short block %d", spec.Height, len(raw))
		}
	}
}

func TestMainnetCanonicalBlock10006Size(t *testing.T) {
	for _, spec := range mainnetCanonicalBlockSpecs {
		if spec.Height != 10006 {
			continue
		}
		raw, err := buildMainnetCanonicalBlockRaw(spec)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) != 213 {
			t.Fatalf("height 10006 block len=%d want 213", len(raw))
		}
		return
	}
	t.Fatal("missing height 10006 spec")
}

func TestMainnetCanonicalFieldBlocksConnect(t *testing.T) {
	blocks, err := mainnetCanonicalFieldBlocks()
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 7 {
		t.Fatalf("got %d canonical blocks", len(blocks))
	}
	for _, e := range blocks {
		if e.Height < 1 || e.Height > 10006 {
			t.Fatalf("unexpected height %d", e.Height)
		}
		if strings.TrimSpace(e.Hex) == "" {
			t.Fatalf("empty hex height %d", e.Height)
		}
	}
}
