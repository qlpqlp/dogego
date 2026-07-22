// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"

	"dogego/wallet"
)

func TestWalletTxRowToListEntryAbandoned(t *testing.T) {
	entry := walletTxRowToListEntry("", nil, nil, nil, nil, nil, "a", walletTxRow{
		txid: "aa", category: "send", abandoned: true, confirmations: 0,
	})
	if entry["abandoned"] != true {
		t.Fatalf("abandoned %#v", entry["abandoned"])
	}
}

func TestExecRemovePrunedFundsWallet(t *testing.T) {
	txid := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	removed := false
	paths := &DataPaths{
		WalletAddress: func() string { return "DAddr" },
		WalletRemoveAbandoned: func(id string) bool {
			if id == txid {
				removed = true
				return true
			}
			return false
		},
	}
	_, code, msg := execRemovePrunedFunds(paths, []json.RawMessage{mustJSON(t, txid)})
	if code != 0 || !removed {
		t.Fatalf("code=%d msg=%q removed=%v", code, msg, removed)
	}
}

func TestExecRemovePrunedFundsPrunedImport(t *testing.T) {
	txid := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	removed := false
	paths := &DataPaths{
		WalletAddress: func() string { return "DAddr" },
		WalletRemovePrunedImport: func(id string) bool {
			if id == txid {
				removed = true
				return true
			}
			return false
		},
	}
	_, code, msg := execRemovePrunedFunds(paths, []json.RawMessage{mustJSON(t, txid)})
	if code != 0 || !removed {
		t.Fatalf("code=%d msg=%q removed=%v", code, msg, removed)
	}
}

func TestExecRemovePrunedFundsNoWallet(t *testing.T) {
	txid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	_, code, _ := execRemovePrunedFunds(nil, []json.RawMessage{mustJSON(t, txid)})
	if code != -8 {
		t.Fatalf("code %d want -8", code)
	}
}

func TestWalletListAbandonedRows(t *testing.T) {
	txid := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	spk := []byte{0x76, 0xa9, 0x14, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x88, 0xac}
	paths := &DataPaths{
		WalletAddress:     func() string { return "DAddr" },
		WalletP2PKHScript: func() []byte { return spk },
		WalletListAbandoned: func() []wallet.AbandonedTx {
			return []wallet.AbandonedTx{{
				TxID: txid, Category: "send", AmountKoinu: -5e7, Address: "DAddr", Time: 1_700_000_001,
			}}
		},
	}
	rows, code, msg := walletCollectTransactions("testnet", paths, nil, nil, nil, 0)
	if code != 0 {
		t.Fatalf("collect: %d %s", code, msg)
	}
	found := false
	for _, r := range rows {
		if r.txid == txid && r.abandoned {
			found = true
			if r.time != 1_700_000_001 {
				t.Fatalf("time %d", r.time)
			}
		}
	}
	if !found {
		t.Fatalf("rows %#v", rows)
	}
}
