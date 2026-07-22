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

func TestImportPrivKeyAddsCosignerWithoutChangingSpendAddress(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "wallet.json")
	w, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	spendAddr := w.Address()

	sk := make([]byte, 32)
	sk[0] = 42
	cosignerWIF, err := chain.EncodeWIF(sk, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.ImportPrivKey(cosignerWIF, p.PrivKeyWIFVersion, p.PubkeyHashAddrID); err != nil {
		t.Fatal(err)
	}
	if w.Address() != spendAddr {
		t.Fatalf("spend address changed: %s -> %s", spendAddr, w.Address())
	}
	wifs, err := w.AllWIFs(p.PrivKeyWIFVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(wifs) != 2 {
		t.Fatalf("wifs %d", len(wifs))
	}

	w2, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if len(w2.extraPrivHex) != 1 {
		t.Fatalf("extra keys %d", len(w2.extraPrivHex))
	}
}
