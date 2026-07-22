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

func TestImportMultiLabel(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, _ := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	paths := &DataPaths{
		WalletAddress:     func() string { return w.Address() },
		WalletImportWatch: func(script []byte) error { return w.AddWatchScript(script) },
		WalletSetLabel:    func(addr, label string) error { return w.SetLabel(addr, label) },
		WalletGetLabel:    func(addr string) string { return w.Label(addr) },
	}
	reqObj := map[string]interface{}{
		"scriptPubKey": map[string]string{"address": w.Address()},
		"label":        "vault",
	}
	reqBytes, _ := json.Marshal(reqObj)
	out, code, msg := execImportMultiWallet("test", paths, nil, nil, []json.RawMessage{json.RawMessage("[" + string(reqBytes) + "]")})
	if code != 0 {
		t.Fatalf("importmulti: %s", msg)
	}
	rows := out.([]map[string]interface{})
	if len(rows) != 1 || rows[0]["success"] != true {
		t.Fatalf("result %#v", out)
	}
	if w.Label(w.Address()) != "vault" {
		t.Fatalf("label %q", w.Label(w.Address()))
	}
}
