//go:build datadir_diag

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

// Run: go test -tags datadir_diag ./node -run TestDatadirContiguousDiag -v
func TestDatadirContiguousDiag(t *testing.T) {
	chainDir := filepath.Join("..", "dogedata", "mainnet")
	gen, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderChain(chainDir, gen[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	bs := &BlockStoreCtx{Journal: j, Raw: raw, Params: p}
	cont := bs.ContiguousRawHeight()
	t.Logf("contiguous=%d", cont)
	for h := int64(9975); h <= 10010; h++ {
		ok := store.HasStoredBodyAtHeight(j, raw, h, p.Net)
		if !ok || h == cont || h == cont+1 {
			t.Logf("height %d stored=%v", h, ok)
		}
	}
	searchStart := store.LowestMissingSearchStart(j, raw, cont, p.Net)
	low, err := store.LowestMissingBlockHeightFrom(j, raw, searchStart, 534000, p.Net)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("searchStart=%d lowest_missing=%d", searchStart, low)
	bs.RefreshContiguousTip()
	t.Logf("after refresh contiguous=%d", bs.ContiguousRawHeight())
	for _, h := range []int64{6855, 6856, 6857, 6858, 10004, 10005} {
		ok := store.HasStoredBodyAtHeight(j, raw, h, p.Net)
		minB := store.MinRawBlockBytes(p.Net, h)
		var size int
		if h80, err := j.ReadHeaderAt(h); err == nil {
			if b, err := raw.Get(pow.BlockHashLE(h80)); err == nil {
				size = len(b)
			}
		}
		t.Logf("height %d stored=%v size=%d min=%d", h, ok, size, minB)
	}
}
