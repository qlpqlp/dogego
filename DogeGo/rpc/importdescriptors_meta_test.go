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

func TestImportDescriptorsInternalTimestamp(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := w.NewReceiveAddress()
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		WalletDefaultAddress: func() string { return w.DefaultAddress() },
		WalletImportWatch:    func(script []byte) error { return w.AddWatchScript(script) },
		WalletAddImportedDescriptor: func(desc string, ts int64, internal, spendable bool) error {
			return w.AddImportedDescriptor(desc, ts, internal, spendable)
		},
	}
	elem, _ := json.Marshal(map[string]interface{}{
		"desc":       "pkh(" + addr + ")",
		"internal":   true,
		"timestamp":  int64(1600000000),
	})
	arr, _ := json.Marshal([]json.RawMessage{json.RawMessage(elem)})
	res, code, msg := execImportDescriptors("testnet", paths, nil, nil, []json.RawMessage{arr, json.RawMessage(`{"rescan":false}`)})
	if code != 0 {
		t.Fatalf("import: %d %s", code, msg)
	}
	rows := w.ListDescriptors(p.PubkeyHashAddrID, p.ScriptHashAddrID)
	desc := "pkh(" + addr + ")"
	var found bool
	for _, r := range rows {
		if r.Desc == desc && r.Internal && r.Timestamp == 1600000000 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected internal+timestamp descriptor, got %+v res=%v", rows, res)
	}
}
