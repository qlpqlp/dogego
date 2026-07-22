// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
)

func TestImportPrivKeyReplacesAddress(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w1, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	addr1 := w1.Address()

	sk := make([]byte, 32)
	sk[0] = 42
	wif, err := chain.EncodeWIF(sk, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := w1.ImportPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID); err != nil {
		t.Fatal(err)
	}
	if w1.Address() != addr1 {
		t.Fatal("spend address should stay when importing a different cosigner key")
	}
	wifs, err := w1.AllWIFs(p.PrivKeyWIFVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(wifs) != 2 {
		t.Fatalf("expected spend + cosigner WIFs, got %d", len(wifs))
	}

	w2, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if w2.Address() != addr1 {
		t.Fatalf("reload addr %s != %s", w2.Address(), addr1)
	}
	if len(w2.extraPrivHex) != 1 {
		t.Fatalf("extra keys %d", len(w2.extraPrivHex))
	}
	_ = os.Remove(path + ".tmp")
}

func TestDefaultPayTxFeeOnCreate(t *testing.T) {
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
	if w.PayTxFee() != DefaultPayTxFeeDOGE {
		t.Fatalf("new wallet paytxfee %v want %v", w.PayTxFee(), DefaultPayTxFeeDOGE)
	}
}

func TestPayTxFeePersist(t *testing.T) {
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
	if err := w.SetPayTxFee(0.05); err != nil {
		t.Fatal(err)
	}
	w2, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if w2.PayTxFee() != 0.05 {
		t.Fatalf("paytxfee %v", w2.PayTxFee())
	}
}
