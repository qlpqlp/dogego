// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
)

func TestHasStoredBodyRejectsMainnetStub(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := MakeTestBlockRaw(t, g80[:])
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw); err != nil {
		t.Fatal(err)
	}
	if !HasStoredBodyAtHeight(j, raw, 0, 0) {
		t.Fatal("testnet stub at 0 should count")
	}
	if HasStoredBodyAtHeight(j, raw, 0, chain.MainnetDogecoin) {
		t.Fatal("190B test stub must not count as mainnet body")
	}
}

func TestMinRawBlockBytesEarlyMainnet(t *testing.T) {
	if got := MinRawBlockBytes(chain.MainnetDogecoin, 1); got != 140 {
		t.Fatalf("height 1 min=%d want 140", got)
	}
	if got := MinRawBlockBytes(chain.MainnetDogecoin, 9_999); got != 140 {
		t.Fatalf("height 9999 min=%d want 140", got)
	}
	if got := MinRawBlockBytes(chain.MainnetDogecoin, 10_006); got != 140 {
		t.Fatalf("height 10006 min=%d want 140 (real coinbase-only block is 213 B)", got)
	}
	if 213 < MinRawBlockBytes(chain.MainnetDogecoin, 10_006) {
		t.Fatal("213 B real block at 10006 must pass size floor")
	}
	if got := MinRawBlockBytes(chain.MainnetDogecoin, 0); got != 200 {
		t.Fatalf("genesis min=%d want 200", got)
	}
}
