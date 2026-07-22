// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"

	"dogego/chain"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

func TestListUnspentIncludeUnsafe(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr, _ := chain.RandomP2PKHAddress(p)
	pk := p2pkhScriptForTest(t, p, addr)
	utxo := store.NewUtxoCache()
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: pk}},
	}
	_ = utxo.ApplyBlock(&wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{coin}}, 0)
	j := &memJournal{tip: 0, count: 1, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return addr },
		WalletP2PKHScript: func() []byte { return pk },
	}
	falseJ, _ := json.Marshal(false)
	res, code, msg := execListUnspent("testnet", paths, j, nil, nil, []json.RawMessage{
		json.RawMessage(`0`),
		json.RawMessage(`9999999`),
		json.RawMessage(`[]`),
		falseJ,
	})
	if code != 0 {
		t.Fatalf("listunspent: %d %s", code, msg)
	}
	arr := res.([]interface{})
	if len(arr) != 0 {
		t.Fatalf("include_unsafe=false minconf=0 want 0 got %d", len(arr))
	}
}
