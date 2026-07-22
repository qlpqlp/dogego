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
	"dogego/wallet"
)

func TestImportPrivKeyLabel(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	path := filepath.Join(t.TempDir(), "wallet.json")
	w, _ := wallet.LoadOrCreate(path, p.PubkeyHashAddrID)
	cosigner, err := chain.EncodeWIF([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := addressFromWIF("test", cosigner)
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		WalletImportPrivKey: func(wif string) error { return w.ImportPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID) },
		WalletSetLabel:      func(a, lbl string) error { return w.SetLabel(a, lbl) },
		WalletGetLabel:      func(a string) string { return w.Label(a) },
	}
	wifJ, _ := json.Marshal(cosigner)
	lblJ, _ := json.Marshal("imported-cosigner")
	_, code, msg := execImportPrivKey("test", paths, nil, nil, []json.RawMessage{wifJ, lblJ, json.RawMessage(`false`)})
	if code != 0 {
		t.Fatalf("importprivkey: %d %s", code, msg)
	}
	if rpcWalletGetLabel(paths, addr) != "imported-cosigner" {
		t.Fatalf("label %q", rpcWalletGetLabel(paths, addr))
	}
}
