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

func TestImportDescriptorsSpendablePKH(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	sec := make([]byte, 32)
	sec[0] = 0x33
	wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	importAddr, err := addressFromWIF("testnet", wif)
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		WalletDefaultAddress: func() string { return w.DefaultAddress() },
		WalletImportPrivKey: func(s string) error {
			return w.ImportPrivKey(s, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
		},
		WalletImportWatch: func(script []byte) error { return w.AddWatchScript(script) },
		WalletSetLabel:    func(addr, label string) error { return w.SetLabel(addr, label) },
		WalletWIFForAddress: func(addr string) (string, error) {
			priv, err := w.PrivKeyForAddress(addr)
			if err != nil {
				return "", err
			}
			return chain.EncodeWIF(priv.Serialize(), p.PrivKeyWIFVersion, true)
		},
	}
	elem, _ := json.Marshal(map[string]interface{}{
		"desc": "pkh(" + importAddr + ")",
		"keys": []string{wif},
	})
	arr, _ := json.Marshal([]json.RawMessage{json.RawMessage(elem)})
	res, code, msg := execImportDescriptors("testnet", paths, nil, nil, []json.RawMessage{arr})
	if code != 0 {
		t.Fatalf("importdescriptors: %d %s", code, msg)
	}
	rows, ok := res.([]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("result %#v", res)
	}
	row := rows[0].(map[string]interface{})
	if row["success"] != true {
		t.Fatalf("row %#v", row)
	}
	info, code2, msg2 := execGetDescriptorInfo("testnet", paths, []json.RawMessage{json.RawMessage(`"pkh(` + importAddr + `)"`)})
	if code2 != 0 {
		t.Fatalf("getdescriptorinfo: %d %s", code2, msg2)
	}
	m := info.(map[string]interface{})
	if m["issolvable"] != true || m["hasprivatekeys"] != true {
		t.Fatalf("expected solvable descriptor %#v", m)
	}
}
