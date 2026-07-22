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

func TestGetAddressInfoMine(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	wif, err := w.WIFExport(p.PrivKeyWIFVersion)
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		WalletAddress: func() string { return w.Address() },
		WalletWIF:     func() string { return wif },
	}
	addrJ, _ := json.Marshal(w.Address())
	res, code, msg := execGetAddressInfo("test", paths, []json.RawMessage{addrJ})
	if code != 0 {
		t.Fatalf("getaddressinfo: %s", msg)
	}
	m := res.(map[string]interface{})
	if m["ismine"] != true || m["iswatchonly"] != false {
		t.Fatalf("mine flags %#v", m)
	}
	if m["pubkey"] == nil || m["pubkey"] == "" {
		t.Fatal("expected pubkey")
	}
}

func TestGetAddressInfoInvalid(t *testing.T) {
	raw, _ := json.Marshal("not-an-address")
	_, code, _ := execGetAddressInfo("test", nil, []json.RawMessage{raw})
	if code != -5 {
		t.Fatalf("code %d want -5", code)
	}
}
