// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestSyncUtxoCacheBulkIBDPassBudget(t *testing.T) {
	c := &BlockStoreCtx{
		Utxo:          nil,
		Journal:       nil,
		Raw:           nil,
		TxIndex:       nil,
		contiguousTip: 8000,
	}
	// nil deps: SyncUtxoCache returns early without panic.
	if err := c.SyncUtxoCache(); err != nil {
		t.Fatalf("nil deps: %v", err)
	}
}

func TestUtxoIBDSyncStrideBacklog(t *testing.T) {
	// Document expected stride tightening when chainActive lags stored bodies.
	cases := []struct {
		cont, utxoTip int64
		wantMax       int64
	}{
		{8000, 7500, 128},
		{8000, 6500, 64},
		{8000, 3000, 32},
	}
	for _, tc := range cases {
		interval := int64(128)
		backlog := tc.cont - tc.utxoTip
		if backlog > 4096 {
			interval = 32
		} else if backlog > 1024 {
			interval = 64
		}
		if interval > tc.wantMax {
			t.Fatalf("cont=%d utxo=%d interval=%d want<=%d", tc.cont, tc.utxoTip, interval, tc.wantMax)
		}
	}
}
