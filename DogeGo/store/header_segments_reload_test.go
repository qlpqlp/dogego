// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"testing"
)

// TestHeaderSegmentReloadManifestFromDisk verifies reloadManifestFromDisk picks up tip written by appendBatch.
func TestHeaderSegmentReloadManifestFromDisk(t *testing.T) {
	dir := t.TempDir()
	gen := make([]byte, 80)
	j, err := OpenHeaderChain(dir, gen)
	if err != nil {
		t.Fatal(err)
	}
	batch := make([]byte, 80*3)
	for i := 0; i < 3; i++ {
		copy(batch[i*80:(i+1)*80], gen)
		binaryPutNonce(batch[i*80:], uint32(i+1))
	}
	if err := j.AppendWireHeaderBatch(batch); err != nil {
		t.Fatal(err)
	}
	l, err := openHeaderSegmentLayout(dir)
	if err != nil {
		t.Fatal(err)
	}
	tip := l.tipHeightLocked()
	l.manifest.TipHeight = -1
	if err := l.reloadManifestFromDisk(); err != nil {
		t.Fatal(err)
	}
	if l.tipHeightLocked() != tip {
		t.Fatalf("reloaded tip=%d want %d", l.tipHeightLocked(), tip)
	}
}
