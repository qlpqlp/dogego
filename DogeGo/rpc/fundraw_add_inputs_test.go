// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"dogego/chain"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

func TestFundRawTransactionAddInputsFalse(t *testing.T) {
	utxo := store.NewUtxoCache()
	spendPK := p2pkhScript(t, chain.RebootTestnet)
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 10_000_000_000, PkScript: spendPK}},
	}
	pb := &wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{coin}}
	if err := utxo.ApplyBlock(pb, 0); err != nil {
		t.Fatal(err)
	}
	coinID := txidToRPC(coin.TxHash())
	prev, _ := decodeRPCPrevHashHex(coinID)
	tx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: prev,
			PrevIdx:  0,
			Sequence: wire.MaxBIP125RBFSequence,
		}},
		Vout: []wire.TxOut{{Value: 1_000_000_000, PkScript: spendPK}},
	}
	raw, _ := tx.Serialize()
	tp, _ := chain.ParamsFor(chain.RebootTestnet)
	changeAddr, _ := chain.RandomP2PKHAddress(tp)
	opts, _ := json.Marshal(map[string]interface{}{
		"changeAddress": changeAddr,
		"add_inputs":    false,
	})
	paths := &DataPaths{Utxo: utxo, WalletP2PKHScript: func() []byte { return spendPK }}
	res, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`"` + hex.EncodeToString(raw) + `"`),
		opts,
	})
	if code != 0 {
		t.Fatalf("add_inputs false with existing input: %d %s", code, msg)
	}
	if res == nil {
		t.Fatal("nil")
	}
}
