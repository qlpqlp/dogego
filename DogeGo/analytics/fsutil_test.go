// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package analytics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChainStoreBytesIncludesHeaderSegmentsAndUtxo(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "headers"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "headers", "seg.dat"), make([]byte, 1000), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "headers_aux.bin"), make([]byte, 2000), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "rawblocks"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rawblocks", "blk00000.dat"), make([]byte, 3000), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "utxo.cache"), make([]byte, 4000), 0o600); err != nil {
		t.Fatal(err)
	}
	// Noise that must not inflate chain total (wallet / analytics).
	if err := os.WriteFile(filepath.Join(dir, "wallet.json"), make([]byte, 50000), 0o600); err != nil {
		t.Fatal(err)
	}
	h, r, tx, total := ChainStoreBytes(dir)
	if h != 3000 {
		t.Fatalf("headers=%d want 3000", h)
	}
	if r != 3000 {
		t.Fatalf("rawblocks=%d want 3000", r)
	}
	if tx != 0 {
		t.Fatalf("txindex=%d want 0", tx)
	}
	if total != 10000 {
		t.Fatalf("total=%d want 10000 (headers+raw+utxo, not wallet)", total)
	}
}
