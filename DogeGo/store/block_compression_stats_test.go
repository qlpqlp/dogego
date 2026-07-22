// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"testing"
)

func TestCompressionStats_bundledZstd(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStoreWithOpts(dir, BlockStorageOpts{Layout: BlockLayoutBundled, Zstd: true})
	if err != nil {
		t.Fatal(err)
	}
	payload, hash := TestMinimalBlock()
	if err := raw.Put(hash, payload); err != nil {
		t.Fatal(err)
	}
	st, err := raw.CachedCompressionStats(0)
	if err != nil {
		t.Fatal(err)
	}
	if st.BlockCount != 1 {
		t.Fatalf("count %d", st.BlockCount)
	}
	if st.LogicalBytes != int64(len(payload)) {
		t.Fatalf("logical %d want %d", st.LogicalBytes, len(payload))
	}
	if st.StoredPayloadBytes <= 0 {
		t.Fatal("stored bytes")
	}
	if st.CompressionRatio <= 0 {
		t.Fatalf("ratio %f", st.CompressionRatio)
	}
}
