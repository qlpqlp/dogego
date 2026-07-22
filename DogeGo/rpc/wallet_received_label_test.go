// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/store"
	"dogego/wallet"
)

func TestListReceivedByLabel(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, _ := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	_ = w.SetLabel(w.Address(), "tips")
	paths := &DataPaths{
		WalletAddress:  func() string { return w.Address() },
		WalletGetLabel: func(addr string) string { return w.Label(addr) },
	}
	labelJ, _ := json.Marshal("tips")
	amt, code, msg := execGetReceivedByLabelWallet("test", paths, &memJournal{}, nil, []json.RawMessage{labelJ})
	if code != 0 {
		t.Fatalf("getreceivedbylabel: %s", msg)
	}
	if amt.(float64) != 0 {
		t.Fatalf("amount %v", amt)
	}
	minConfJ, _ := json.Marshal(0)
	emptyJ, _ := json.Marshal(true)
	list, code, msg := execListReceivedByLabelWallet("test", paths, &memJournal{}, nil, []json.RawMessage{minConfJ, emptyJ})
	if code != 0 {
		t.Fatalf("listreceivedbylabel: %s", msg)
	}
	rows := list.([]interface{})
	if len(rows) != 1 {
		t.Fatalf("rows %#v", list)
	}
	row := rows[0].(map[string]interface{})
	if row["label"] != "tips" {
		t.Fatalf("label %#v", row["label"])
	}
	txids, ok := row["txids"].([]interface{})
	if !ok {
		t.Fatalf("txids %#v", row["txids"])
	}
	if len(txids) != 0 {
		t.Fatalf("txids %#v want empty without UTXO", txids)
	}
}

func TestWalletReceivedAggTxids(t *testing.T) {
	a := &walletReceivedAgg{}
	a.addMatch(walletUtxoMatch{row: store.UtxoDumpRow{TxID: "BB", Value: 1}, confirmations: 2, address: "x"})
	a.addMatch(walletUtxoMatch{row: store.UtxoDumpRow{TxID: "aa", Value: 2}, confirmations: 1, address: "x"})
	a.addMatch(walletUtxoMatch{row: store.UtxoDumpRow{TxID: "AA", Value: 3}, confirmations: 1, address: "x"})
	ids := walletReceivedAggTxids(a)
	if len(ids) != 2 {
		t.Fatalf("txids len %d want 2 (dedupe)", len(ids))
	}
	if ids[0] != "aa" || ids[1] != "bb" {
		t.Fatalf("sorted txids %#v", ids)
	}
}
