// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import "testing"

func TestLookupSendFee(t *testing.T) {
	w := &Disk{
		scannedTx: []ScannedTx{
			{TxID: "aa", Category: "send", FeeKoinu: 1_000_000},
			{TxID: "bb", Category: "receive", AmountKoinu: 5},
		},
	}
	fee, ok := w.LookupSendFee("AA")
	if !ok || fee != 1_000_000 {
		t.Fatalf("lookup aa: ok=%v fee=%d", ok, fee)
	}
	if _, ok := w.LookupSendFee("bb"); ok {
		t.Fatal("receive row should not match")
	}
}
