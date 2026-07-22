// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dogego/wire"
)

func TestLoadFromJSONLFile(t *testing.T) {
	u := NewUtxoCache()
	u.ApplyBlock(&wire.ParsedBlock{
		Txs: []*wire.Tx{{
			Version: 1,
			Vin:     []wire.TxIn{{PrevIdx: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 99, PkScript: []byte{0x51}}},
		}},
	}, 0)
	rows := u.DumpRows()
	if len(rows) != 1 {
		t.Fatalf("rows %d", len(rows))
	}
	path := filepath.Join(t.TempDir(), "utxo.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	if err := enc.Encode(map[string]interface{}{
		"txid":         rows[0].TxID,
		"vout":         rows[0].Vout,
		"value":        rows[0].Value,
		"height":       rows[0].Height,
		"scriptPubKey": hex.EncodeToString(rows[0].PkScript),
	}); err != nil {
		t.Fatal(err)
	}
	f.Close()
	u2 := NewUtxoCache()
	n, err := u2.LoadFromJSONLFile(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || u2.Count() != 1 {
		t.Fatalf("n=%d count=%d", n, u2.Count())
	}
}
