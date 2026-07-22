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

func TestFundRawTransactionReplaceableFalse(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr, _ := chain.RandomP2PKHAddress(p)
	_, h160, _ := chain.Base58CheckDecode(addr)
	pk := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pk = append(pk, 0x88, 0xac)

	utxo := store.NewUtxoCache()
	fund := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 50_000_000_000, PkScript: pk}},
	}
	_ = utxo.ApplyBlock(&wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{fund}}, 0)

	spend := &wire.Tx{Version: 1, Vin: []wire.TxIn{}, Vout: []wire.TxOut{{Value: 1_000_000_000, PkScript: pk}}}
	raw, _ := spend.Serialize()
	hexJ, _ := json.Marshal(hex.EncodeToString(raw))
	optsJ, _ := json.Marshal(map[string]interface{}{"changeAddress": addr, "replaceable": false})

	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return addr },
		WalletP2PKHScript: func() []byte { return pk },
	}
	res, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{hexJ, optsJ})
	if code != 0 {
		t.Fatalf("fund: %d %s", code, msg)
	}
	m := res.(map[string]interface{})
	outHex, _ := m["hex"].(string)
	txRaw, _ := hex.DecodeString(outHex)
	tx, err := wire.DeserializeTx(txRaw)
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range tx.Vin {
		if in.Sequence != 0xffffffff {
			t.Fatalf("sequence %#x want final", in.Sequence)
		}
	}
}
