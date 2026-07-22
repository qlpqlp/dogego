// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestWalletTxDetailsMultiReceive(t *testing.T) {
	paths := &DataPaths{}
	rows := []walletTxRow{
		{txid: "aa", category: "receive", amountKoinu: 1e8, vout: 0, address: "D1"},
		{txid: "aa", category: "receive", amountKoinu: 2e8, vout: 1, address: "D2"},
	}
	matched := walletTxRowsForTxid(rows, "aa", paths, false)
	if len(matched) != 2 {
		t.Fatalf("matched %d", len(matched))
	}
	details := walletTxDetailsFromRows(paths, matched)
	if len(details) != 2 {
		t.Fatalf("details %d", len(details))
	}
	if walletTxEntryAmountKoinu(matched) != 3e8 {
		t.Fatalf("sum %d", walletTxEntryAmountKoinu(matched))
	}
}
