// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/wire"
)

func TestClassifyAndIngestSPVMatchedTx(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	pk := w.P2PKHScript()
	if len(pk) == 0 {
		t.Fatal("no p2pkh")
	}
	tx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: [32]byte{9}, PrevIdx: 0, Script: []byte{0x00}, Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 1_000_000, PkScript: pk}},
	}
	rows := ClassifyTxAgainstTracked(tx, 42, w.TrackedScripts(), p.PubkeyHashAddrID, p.ScriptHashAddrID, nil)
	if len(rows) != 1 || rows[0].Category != "receive" || rows[0].AmountKoinu != 1_000_000 {
		t.Fatalf("rows %#v", rows)
	}
	if err := w.IngestSPVMatchedTx(tx, 42, p.PubkeyHashAddrID, p.ScriptHashAddrID); err != nil {
		t.Fatal(err)
	}
	got := w.ListScannedTx()
	if len(got) == 0 || got[0].BlockHeight != 42 {
		t.Fatalf("scanned %#v", got)
	}
}
