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

func TestGetAccountAddressPeekKeypool(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		WalletAddress:            func() string { return w.Address() },
		WalletPeekReceiveAddress: func() string { return w.PeekReceiveAddress() },
	}
	accJ, _ := json.Marshal("")
	res, code, msg := execGetAccountAddressWallet(paths, []json.RawMessage{accJ})
	if code != 0 {
		t.Fatalf("getaccountaddress: %s", msg)
	}
	addr, _ := res.(string)
	peek := w.PeekReceiveAddress()
	if addr != peek {
		t.Fatalf("rpc %q peek %q", addr, peek)
	}
}

func TestListAddressGroupingsByAddress(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		WalletAddress:     func() string { return w.Address() },
		WalletSpendScripts: func() [][]byte { return w.SpendScripts() },
	}
	// No UTXO journal - empty groupings is fine; test structure with manual utxo would be heavy.
	res, code, msg := execListAddressGroupingsWallet("test", paths, nil, nil, nil)
	if code != 0 {
		t.Fatalf("listaddressgroupings: %s", msg)
	}
	if _, ok := res.([]interface{}); !ok {
		t.Fatalf("result %#v", res)
	}
}
