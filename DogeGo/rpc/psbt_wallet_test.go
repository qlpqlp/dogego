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
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

func TestWalletCreateAndProcessPsbt(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	spk := w.P2PKHScript()
	utxo := store.NewUtxoCache()
	fund := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5e8, PkScript: spk}},
	}
	if err := utxo.ApplyBlock(&wire.ParsedBlock{Txs: []*wire.Tx{fund}}, 0); err != nil {
		t.Fatal(err)
	}
	paths := walletTestPaths(w, p)
	paths.Utxo = utxo
	dest, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	outs, _ := json.Marshal(map[string]float64{dest: 1.0})
	createRes, code, msg := execWalletCreateFundedPsbt("test", paths, nil, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`[]`),
		outs,
	})
	if code != 0 {
		t.Fatalf("create %d %s", code, msg)
	}
	cm := createRes.(map[string]interface{})
	b64, ok := cm["psbt"].(string)
	if !ok || b64 == "" {
		t.Fatalf("create result %#v", cm)
	}
	procRes, code2, msg2 := execWalletProcessPsbt("test", paths, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`"` + b64 + `"`),
	})
	if code2 != 0 {
		t.Fatalf("process %d %s", code2, msg2)
	}
	pm := procRes.(map[string]interface{})
	if pm["complete"] != true {
		t.Fatalf("complete %#v", pm["complete"])
	}
	if _, ok := pm["hex"].(string); !ok {
		t.Fatalf("hex %#v", pm["hex"])
	}
}
