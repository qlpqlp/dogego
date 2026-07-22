// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"testing"

	"dogego/chain"
)

func TestAvoidReuseScriptIndex(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := LoadOrCreate(t.TempDir()+"/wallet.json", p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	w.SetNetAddrVersions(p.PubkeyHashAddrID, p.ScriptHashAddrID)
	addr, err := w.NewReceiveAddress()
	if err != nil {
		t.Fatal(err)
	}
	_, h160, err := chain.Base58CheckDecode(addr)
	if err != nil {
		t.Fatal(err)
	}
	spk := chain.P2PKHScriptFromPubKeyHash(h160)

	w.mu.Lock()
	w.avoidReuse = true
	w.scannedTx = append(w.scannedTx, ScannedTx{
		TxID: "abc", Category: "receive", Address: addr, AmountKoinu: 1e8, Vout: 0, BlockHeight: 1,
	})
	w.rebuildUsedRecvScriptsLocked()
	w.mu.Unlock()

	if !w.IsRecvScriptReused(spk) {
		t.Fatal("expected receive script marked reused")
	}
	if w.IsRecvScriptReused(chain.P2PKHScriptFromPubKeyHash([20]byte{1})) {
		t.Fatal("unexpected reused on unrelated script")
	}

	w.mu.Lock()
	w.avoidReuse = false
	w.mu.Unlock()
	if w.IsRecvScriptReused(spk) {
		t.Fatal("avoid_reuse off should not report reused")
	}
}
