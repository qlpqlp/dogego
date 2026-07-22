// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import "testing"

func TestMaxScannedBlockHeight(t *testing.T) {
	w := &Disk{
		scannedTx: []ScannedTx{
			{BlockHeight: 10},
			{BlockHeight: 42},
			{BlockHeight: 5},
		},
	}
	if got := w.MaxScannedBlockHeight(); got != 42 {
		t.Fatalf("max=%d want 42", got)
	}
	w.scannedTx = nil
	if got := w.MaxScannedBlockHeight(); got != -1 {
		t.Fatalf("empty max=%d want -1", got)
	}
}
