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

func TestExecFundRawTransactionFromUtxo(t *testing.T) {
	utxo := store.NewUtxoCache()
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 10_000_000_000, PkScript: p2pkhScript(t, chain.RebootTestnet)}},
	}
	pb := &wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{coin}}
	if err := utxo.ApplyBlock(pb, 0); err != nil {
		t.Fatal(err)
	}
	coinID := txidToRPC(coin.TxHash())
	empty := &wire.Tx{
		Version: 1,
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: p2pkhScript(t, chain.RebootTestnet)}},
	}
	emptyRaw, _ := empty.Serialize()
	tp, _ := chain.ParamsFor(chain.RebootTestnet)
	changeAddr, _ := chain.RandomP2PKHAddress(tp)
	opts, _ := json.Marshal(map[string]string{"changeAddress": changeAddr})
	paths := &DataPaths{Utxo: utxo}
	res, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`"` + hex.EncodeToString(emptyRaw) + `"`),
		json.RawMessage(opts),
	})
	if code != 0 {
		t.Fatalf("fund: %d %s", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("result type %T", res)
	}
	fundedHex, _ := m["hex"].(string)
	funded, err := hex.DecodeString(fundedHex)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := wire.DeserializeTx(funded)
	if err != nil {
		t.Fatal(err)
	}
	if len(tx.Vin) != 1 {
		t.Fatalf("want 1 input, got %d", len(tx.Vin))
	}
	if txidToRPC(tx.Vin[0].PrevHash) != coinID {
		t.Fatalf("unexpected input %s", txidToRPC(tx.Vin[0].PrevHash))
	}
}

func TestApplySubtractFeeFromOutputs(t *testing.T) {
	tx := &wire.Tx{
		Vout: []wire.TxOut{
			{Value: 100_000_000},
			{Value: 50_000_000},
		},
	}
	if err := applySubtractFeeFromOutputs(tx, []int{0}, 1_000_000); err != nil {
		t.Fatal(err)
	}
	if tx.Vout[0].Value != 99_000_000 {
		t.Fatalf("vout0 %d", tx.Vout[0].Value)
	}
	if tx.Vout[1].Value != 50_000_000 {
		t.Fatalf("vout1 unchanged")
	}
}

func p2pkhScript(t *testing.T, net chain.Network) []byte {
	t.Helper()
	p, err := chain.ParamsFor(net)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	_, h160, err := chain.Base58CheckDecode(addr)
	if err != nil {
		t.Fatal(err)
	}
	pk := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	return append(pk, 0x88, 0xac)
}
