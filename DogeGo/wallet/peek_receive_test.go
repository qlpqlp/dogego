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
)

func TestPeekReceiveAddressKeypool(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	peek := w.PeekReceiveAddress()
	issued, err := w.NewReceiveAddress()
	if err != nil {
		t.Fatal(err)
	}
	if peek != issued {
		t.Fatalf("peek %q issued %q", peek, issued)
	}
}

func TestConsumeReceiveKeypoolOnScannedPayment(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	w.SetNetAddrVersions(p.PubkeyHashAddrID, p.ScriptHashAddrID)
	peek := w.PeekReceiveAddress()
	if peek == "" || !w.IsReceiveInKeypool(peek) {
		t.Fatalf("peek %q not in keypool", peek)
	}
	before := w.KeypoolSize()
	w.mu.Lock()
	w.mergeScannedFromHeightLocked(1, []ScannedTx{{
		TxID: "deadbeef", Category: "receive", Address: peek, AmountKoinu: 1e8, Vout: 0, BlockHeight: 1,
	}})
	w.mu.Unlock()
	if w.IsReceiveInKeypool(peek) {
		t.Fatal("payment to peeked address must leave receive keypool")
	}
	if got := w.KeypoolSize(); got != before-1 {
		t.Fatalf("keypool=%d want %d", got, before-1)
	}
	if err := w.ConsumeReceiveKeypoolAddress(peek); err != nil {
		t.Fatal(err)
	}
}

func TestConsumeReceiveKeypoolMarksAvoidReuse(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	w.SetNetAddrVersions(p.PubkeyHashAddrID, p.ScriptHashAddrID)
	peek := w.PeekReceiveAddress()
	_, h160, err := chain.Base58CheckDecode(peek)
	if err != nil {
		t.Fatal(err)
	}
	spk := chain.P2PKHScriptFromPubKeyHash(h160)
	w.mu.Lock()
	w.avoidReuse = true
	w.mergeScannedFromHeightLocked(1, []ScannedTx{{
		TxID: "cafe", Category: "receive", Address: peek, AmountKoinu: 5e7, Vout: 0, BlockHeight: 2,
	}})
	w.mu.Unlock()
	if w.IsReceiveInKeypool(peek) {
		t.Fatal("expected keypool consume")
	}
	if !w.IsRecvScriptReused(spk) {
		t.Fatal("expected avoid_reuse mark after receive to peeked address")
	}
}
