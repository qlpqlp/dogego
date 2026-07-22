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

func TestImportMultiDescPKH(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	addr := w.DefaultAddress()
	desc := "pkh(" + addr + ")"
	req, _ := json.Marshal(map[string]interface{}{
		"desc":      desc,
		"timestamp": 0,
	})
	paths := &DataPaths{
		WalletAddress:               func() string { return addr },
		WalletDefaultAddress:        func() string { return addr },
		WalletImportWatch:           func(script []byte) error { return w.AddWatchScript(script) },
		WalletWatchScripts:          func() [][]byte { return w.WatchScripts() },
		WalletAddImportedDescriptor: w.AddImportedDescriptor,
	}
	batch, _ := json.Marshal([]json.RawMessage{req})
	out, code, msg := execImportMultiWallet("test", paths, nil, nil, []json.RawMessage{batch})
	if code != 0 {
		t.Fatalf("importmulti: %s", msg)
	}
	rows := out.([]map[string]interface{})
	if len(rows) != 1 || rows[0]["success"] != true {
		t.Fatalf("result %#v", out)
	}
	if len(w.WatchScripts()) != 1 {
		t.Fatal("expected watch script")
	}
}
