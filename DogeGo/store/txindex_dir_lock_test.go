// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"sync"
	"testing"

	"dogego/pow"
)

// Two *TxIndex on the same indexes/tx root must not race Windows rename (repair + live connect).
func TestTxIndexConcurrentIndexBlockSharedDir(t *testing.T) {
	dir := t.TempDir()
	a, err := OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	h80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	raw := makeTestBlockRaw(t, h80[:])
	id := pow.BlockHashLE(raw[:80])

	var wg sync.WaitGroup
	errCh := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		ix := a
		if i%2 == 1 {
			ix = b
		}
		go func(ix *TxIndex) {
			defer wg.Done()
			if err := ix.IndexBlock(id, raw); err != nil {
				errCh <- err
			}
		}(ix)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	hit, err := a.LookupHit(txidFromTestBlock(raw))
	if err != nil {
		t.Fatal(err)
	}
	if hit.BlockHashLE != id {
		t.Fatalf("block hash %#v", hit.BlockHashLE)
	}
}
