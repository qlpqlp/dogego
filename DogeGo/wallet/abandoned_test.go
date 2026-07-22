// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"path/filepath"
	"strings"
	"testing"

	"dogego/chain"
)

func TestAbandonTxPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	txid := strings.Repeat("e", 64)
	if err := w.AbandonTx(AbandonedTx{
		TxID: txid, Category: "send", AmountKoinu: -1e8, Address: "DAbandon",
	}); err != nil {
		t.Fatal(err)
	}
	if !w.IsAbandoned(txid) {
		t.Fatal("not abandoned")
	}
	w2, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if len(w2.ListAbandoned()) != 1 {
		t.Fatalf("reload %#v", w2.ListAbandoned())
	}
	if !w2.RemoveAbandoned(txid) {
		t.Fatal("remove failed")
	}
	if w2.IsAbandoned(txid) {
		t.Fatal("still abandoned")
	}
}
