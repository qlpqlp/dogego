// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestExecDogegoImportMnemonic(t *testing.T) {
	var gotMnemonic, gotPass string
	var rescanned int64 = -1
	paths := &DataPaths{
		WalletImportMnemonic: func(m, p string) error {
			gotMnemonic, gotPass = m, p
			return nil
		},
		WalletDefaultAddress: func() string { return "DTestImportMnemonic" },
		WalletHDFormat:       func() string { return "hd" },
		SyncUtxo:             func() error { return nil },
		WalletRescanBlocks: func(start int64) error {
			rescanned = start
			return nil
		},
	}
	mn := strings.Repeat("abandon ", 11) + "about"
	mnJ, _ := json.Marshal(mn)
	passJ, _ := json.Marshal("hunter2")
	res, code, msg := execDogegoImportMnemonic("main", paths, nil, nil, []json.RawMessage{mnJ, passJ, json.RawMessage(`false`)})
	if code != 0 {
		t.Fatalf("import: %d %s", code, msg)
	}
	if gotMnemonic != mn || gotPass != "hunter2" {
		t.Fatalf("mnemonic/pass %q %q", gotMnemonic, gotPass)
	}
	if rescanned >= 0 {
		t.Fatalf("rescan should be skipped, got start %d", rescanned)
	}
	m, ok := res.(map[string]any)
	if !ok || m["ok"] != true || m["address"] != "DTestImportMnemonic" || m["hd"] != true {
		t.Fatalf("result %#v", res)
	}
}

func TestExecDogegoImportMnemonicRescanDefault(t *testing.T) {
	var rescanned int64 = -1
	paths := &DataPaths{
		WalletImportMnemonic: func(m, p string) error { return nil },
		WalletDefaultAddress: func() string { return "DAddr" },
		SyncUtxo:             func() error { return nil },
		WalletRescanBlocks: func(start int64) error {
			rescanned = start
			return nil
		},
	}
	mnJ, _ := json.Marshal("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about")
	_, code, msg := execDogegoImportMnemonic("main", paths, nil, nil, []json.RawMessage{mnJ})
	if code != 0 {
		t.Fatalf("import: %d %s", code, msg)
	}
	if rescanned != 0 {
		t.Fatalf("default rescan from genesis want 0 got %d", rescanned)
	}
}

func TestExecDogegoImportMnemonicNoWallet(t *testing.T) {
	_, code, msg := execDogegoImportMnemonic("main", &DataPaths{}, nil, nil, []json.RawMessage{json.RawMessage(`"x"`)})
	if code != -1 {
		t.Fatalf("code %d msg %s", code, msg)
	}
}

func TestExecDogegoListWalletAddresses(t *testing.T) {
	paths := &DataPaths{
		WalletListAddresses: func() []WalletAddressEntry {
			return []WalletAddressEntry{
				{Address: "DReceive", HDPath: "m/44'/3'/0'/0/0"},
				{Address: "DChange", HDPath: "m/44'/3'/0'/1/0", IsChange: true},
			}
		},
	}
	res, code, msg := execDogegoListWalletAddresses(paths)
	if code != 0 {
		t.Fatalf("list: %d %s", code, msg)
	}
	rows, ok := res.([]WalletAddressEntry)
	if !ok || len(rows) != 2 {
		t.Fatalf("got %#v", res)
	}
}

func TestExecDogegoImportBIP38(t *testing.T) {
	var gotEnc, gotPass string
	paths := &DataPaths{
		WalletImportBIP38: func(enc, pass string) (string, error) {
			gotEnc, gotPass = enc, pass
			return "DSwept", nil
		},
		SyncUtxo: func() error { return nil },
		WalletRescanBlocks: func(int64) error {
			return nil
		},
	}
	encJ, _ := json.Marshal("6PTestKey")
	passJ, _ := json.Marshal("pass")
	res, code, msg := execDogegoImportBIP38("main", paths, nil, nil, []json.RawMessage{encJ, passJ, json.RawMessage(`false`)})
	if code != 0 {
		t.Fatalf("import: %d %s", code, msg)
	}
	if gotEnc != "6PTestKey" || gotPass != "pass" {
		t.Fatalf("enc/pass %q %q", gotEnc, gotPass)
	}
	m, ok := res.(map[string]any)
	if !ok || m["address"] != "DSwept" {
		t.Fatalf("result %#v", res)
	}
}

func TestExecDogegoImportMnemonicInvalidChecksum(t *testing.T) {
	paths := &DataPaths{
		WalletImportMnemonic: func(m, p string) error {
			return errors.New("invalid mnemonic checksum")
		},
	}
	mnJ, _ := json.Marshal(strings.Repeat("abandon ", 11) + "ability")
	_, code, msg := execDogegoImportMnemonic("main", paths, nil, nil, []json.RawMessage{mnJ})
	if code != -8 || !strings.Contains(msg, "checksum") {
		t.Fatalf("code %d msg %q", code, msg)
	}
}

func TestExecDogegoImportBIP38MissingPassphrase(t *testing.T) {
	encJ, _ := json.Marshal("6PTestKey")
	_, code, msg := execDogegoImportBIP38("main", &DataPaths{WalletImportBIP38: func(string, string) (string, error) {
		return "", nil
	}}, nil, nil, []json.RawMessage{encJ})
	if code != -8 || !strings.Contains(msg, "passphrase") {
		t.Fatalf("code %d msg %q", code, msg)
	}
}
