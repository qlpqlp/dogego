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

func TestImportBIP38LotSequence(t *testing.T) {
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
	enc := "6PgNBNNzDkKdhkT6uJntUXwwzQV8Rr2tZcbkDcuC9DZRsS6AtHts4Ypo1j"
	if err := w.ImportBIP38(enc, "MOLON LABE", p.PrivKeyWIFVersion, p.PubkeyHashAddrID); err != nil {
		t.Fatal(err)
	}
	if w.Address() == "" {
		t.Fatal("expected spend address after BIP38 import")
	}
	entries := w.ListAddressEntries(p.PubkeyHashAddrID, p.ScriptHashAddrID)
	if len(entries) < 1 {
		t.Fatal("expected address book entry")
	}
	_ = os.Remove(path)
}
