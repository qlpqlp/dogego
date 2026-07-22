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

func TestListTransactionsFilterByLabel(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, _ := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	_ = w.SetLabel(w.Address(), "savings")
	paths := &DataPaths{
		WalletAddress:  func() string { return w.Address() },
		WalletGetLabel: func(addr string) string { return w.Label(addr) },
	}
	acctJ, _ := json.Marshal("savings")
	countJ, _ := json.Marshal(10)
	skipJ, _ := json.Marshal(0)
	watchJ, _ := json.Marshal(false)
	res, code, msg := execListTransactionsWallet("test", paths, &memJournal{}, nil, nil, nil, []json.RawMessage{acctJ, countJ, skipJ, watchJ})
	if code != 0 {
		t.Fatalf("listtransactions: %s", msg)
	}
	_ = res
	acctJ2, _ := json.Marshal("other")
	res2, _, _ := execListTransactionsWallet("test", paths, &memJournal{}, nil, nil, nil, []json.RawMessage{acctJ2, countJ, skipJ, watchJ})
	if len(res2.([]interface{})) != 0 {
		t.Fatalf("expected empty for unknown label %#v", res2)
	}
}

func TestGetNewAddressSetsLabel(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, _ := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	paths := &DataPaths{
		WalletAddress:    func() string { return w.Address() },
		WalletNewAddress: func() (string, error) { return w.NewReceiveAddress() },
		WalletSetLabel:   func(addr, label string) error { return w.SetLabel(addr, label) },
	}
	labelJ, _ := json.Marshal("receive")
	addr, code, msg := execGetNewAddress("test", paths, []json.RawMessage{labelJ})
	if code != 0 {
		t.Fatalf("getnewaddress: %s", msg)
	}
	if w.Label(addr.(string)) != "receive" {
		t.Fatalf("label %q", w.Label(w.Address()))
	}
}

func TestGetAddressesByLabel(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, _ := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	_ = w.SetLabel(w.Address(), "main")
	paths := &DataPaths{
		WalletAddress:  func() string { return w.Address() },
		WalletGetLabel: func(addr string) string { return w.Label(addr) },
	}
	labelJ, _ := json.Marshal("main")
	res, code, msg := execGetAddressesByLabelWallet("test", paths, []json.RawMessage{labelJ})
	if code != 0 {
		t.Fatalf("getaddressesbylabel: %s", msg)
	}
	rows := res.(map[string]interface{})
	if len(rows) != 1 {
		t.Fatalf("addrs %#v", res)
	}
	got, ok := rows[w.Address()].(map[string]interface{})
	if !ok || got["purpose"] != "receive" {
		t.Fatalf("row %#v", rows[w.Address()])
	}
}

func TestGetAddressesByLabelHDChangePurpose(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	recv, err := w.NewReceiveAddress()
	if err != nil {
		t.Fatal(err)
	}
	chg, err := w.NewChangeAddress()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.SetLabel(recv, "tips"); err != nil {
		t.Fatal(err)
	}
	if err := w.SetLabel(chg, "change-pool"); err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		WalletAddress:         func() string { return w.Address() },
		WalletContainsAddress: func(a string) bool { return w.ContainsAddress(a) },
		WalletKnownAddresses:  func() []string { return w.KnownAddresses(p.PubkeyHashAddrID, p.ScriptHashAddrID) },
		WalletGetLabel:        func(addr string) string { return w.Label(addr) },
		WalletAddressHDPath:   func(a string) (string, bool, bool) { return w.AddressHDPath(a) },
	}
	recvRows, code, msg := execGetAddressesByLabelWallet("test", paths, []json.RawMessage{json.RawMessage(`"tips"`)})
	if code != 0 {
		t.Fatalf("getaddressesbylabel receive: %s", msg)
	}
	recvMap := recvRows.(map[string]interface{})
	if len(recvMap) != 1 {
		t.Fatalf("receive rows %#v", recvMap)
	}
	recvEntry, ok := recvMap[recv].(map[string]interface{})
	if !ok || recvEntry["purpose"] != "receive" {
		t.Fatalf("receive purpose %#v", recvMap[recv])
	}
	chgRows, code, msg := execGetAddressesByLabelWallet("test", paths, []json.RawMessage{json.RawMessage(`"change-pool"`)})
	if code != 0 {
		t.Fatalf("getaddressesbylabel change: %s", msg)
	}
	chgMap := chgRows.(map[string]interface{})
	chgEntry, ok := chgMap[chg].(map[string]interface{})
	if !ok || chgEntry["purpose"] != "send" {
		t.Fatalf("change purpose %#v", chgMap[chg])
	}
}

func TestImportAddressSetsLabel(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, _ := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	paths := &DataPaths{
		WalletAddress:     func() string { return w.Address() },
		WalletImportWatch: func(script []byte) error { return w.AddWatchScript(script) },
		WalletSetLabel:    func(addr, label string) error { return w.SetLabel(addr, label) },
	}
	k1 := w.Address()
	addrJ, _ := json.Marshal(k1)
	labelJ, _ := json.Marshal("donations")
	_, code, msg := execImportAddress("test", paths, &memJournal{}, nil, []json.RawMessage{addrJ, labelJ, json.RawMessage("false")})
	if code != 0 {
		t.Fatalf("importaddress: %s", msg)
	}
	if w.Label(k1) != "donations" {
		t.Fatalf("label %q", w.Label(k1))
	}
}
