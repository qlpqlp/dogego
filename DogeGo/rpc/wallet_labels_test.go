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

func TestSetLabelAndGetAddressInfo(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		WalletAddress:    func() string { return w.Address() },
		WalletGetLabel:   func(addr string) string { return w.Label(addr) },
		WalletSetLabel:   func(addr, label string) error { return w.SetLabel(addr, label) },
		WalletListLabels: func() []string { return w.ListLabels() },
	}
	addrJ, _ := json.Marshal(w.Address())
	labelJ, _ := json.Marshal("savings")
	_, code, msg := execSetLabelWallet("test", paths, []json.RawMessage{addrJ, labelJ})
	if code != 0 {
		t.Fatalf("setlabel: %s", msg)
	}
	res, code, msg := execGetAddressInfo("test", paths, []json.RawMessage{addrJ})
	if code != 0 {
		t.Fatalf("getaddressinfo: %s", msg)
	}
	m := res.(map[string]interface{})
	if m["label"] != "savings" {
		t.Fatalf("label %#v", m["label"])
	}
	labels, code, msg := execListLabelsWallet(paths, nil)
	if code != 0 {
		t.Fatalf("listlabels: %s", msg)
	}
	arr := labels.([]interface{})
	if len(arr) != 1 || arr[0] != "savings" {
		t.Fatalf("listlabels %#v", labels)
	}
	byLabel, code, msg := execGetAddressesByLabelWallet("test", paths, []json.RawMessage{labelJ})
	if code != 0 {
		t.Fatalf("getaddressesbylabel: %s", msg)
	}
	rows := byLabel.(map[string]interface{})
	entry, ok := rows[w.Address()].(map[string]interface{})
	if !ok || entry["purpose"] != "receive" {
		t.Fatalf("getaddressesbylabel %#v", rows[w.Address()])
	}
}

func TestWalletPassphraseUnencrypted(t *testing.T) {
	phraseJ, _ := json.Marshal("secret")
	timeoutJ, _ := json.Marshal(60)
	_, code, msg := execWalletPassphraseUnencrypted([]json.RawMessage{phraseJ, timeoutJ})
	if code != -15 || msg == "" {
		t.Fatalf("code %d msg %q", code, msg)
	}
}
