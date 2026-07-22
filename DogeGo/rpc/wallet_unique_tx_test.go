// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/wallet"
)

func TestWalletUniqueTxCountFromScannedOnly(t *testing.T) {
	paths := &DataPaths{
		WalletAddress: func() string { return "DWalletAddr" },
		WalletListScannedTx: func() []wallet.ScannedTx {
			return []wallet.ScannedTx{
				{TxID: "aa"},
				{TxID: "bb"},
				{TxID: "AA"}, // duplicate case-insensitive
			}
		},
	}
	n := walletUniqueTxCount("testnet", paths, nil, nil, nil)
	if n != 2 {
		t.Fatalf("count=%d want 2 unique txids", n)
	}
}

func TestWalletUniqueTxCountEmptyWallet(t *testing.T) {
	paths := &DataPaths{
		WalletAddress:       func() string { return "" },
		WalletWatchScripts:  func() [][]byte { return nil },
		WalletListScannedTx: func() []wallet.ScannedTx { return nil },
	}
	if n := walletUniqueTxCount("testnet", paths, nil, nil, nil); n != 0 {
		t.Fatalf("count=%d want 0", n)
	}
}
