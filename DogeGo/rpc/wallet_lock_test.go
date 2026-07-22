// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/primitives"
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

func TestFundRawTransactionWalletOnlyInputs(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	walletPK := w.P2PKHScript()
	otherPK := append([]byte(nil), walletPK...)
	otherPK[8] ^= 0x02
	utxo := store.NewUtxoCache()
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout: []wire.TxOut{
			{Value: 5e8, PkScript: walletPK},
			{Value: 9e8, PkScript: otherPK},
		},
	}
	pb := &wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{coin}}
	_ = utxo.ApplyBlock(pb, 0)
	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return w.Address() },
		WalletP2PKHScript: func() []byte { return walletPK },
	}
	unsigned := &wire.Tx{Version: 1, Vin: []wire.TxIn{}, Vout: []wire.TxOut{{Value: 1e8, PkScript: walletPK}}}
	raw, _ := unsigned.Serialize()
	hexJ, _ := json.Marshal(hex.EncodeToString(raw))
	opts, _ := json.Marshal(map[string]interface{}{"changeAddress": w.Address()})
	res, code, msg := execFundRawTransaction("test", paths, nil, nil, nil, []json.RawMessage{hexJ, opts})
	if code != 0 {
		t.Fatalf("fund: %s", msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("result %#v", res)
	}
	funded, _ := m["hex"].(string)
	rawFunded, err := hex.DecodeString(funded)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := wire.DeserializeTx(rawFunded)
	if err != nil {
		t.Fatal(err)
	}
	if len(tx.Vin) != 1 {
		t.Fatalf("vins %d", len(tx.Vin))
	}
	e, ok := utxo.LookupOutpoint(tx.Vin[0].PrevHash, tx.Vin[0].PrevIdx)
	if !ok || string(e.PkScript) != string(walletPK) {
		t.Fatal("funded from non-wallet utxo")
	}
}

func TestLockUnspentExcludesFund(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	walletPK := w.P2PKHScript()
	utxo := store.NewUtxoCache()
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 8e8, PkScript: walletPK}},
	}
	pb := &wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{coin}}
	_ = utxo.ApplyBlock(pb, 0)
	coinID := coin.TxHash()
	txid := txidToRPC(coinID)
	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return w.Address() },
		WalletP2PKHScript: func() []byte { return walletPK },
		WalletListLocked:  func() []wallet.LockedOutpoint { return w.ListLockedOutpoints() },
		WalletSetLocked: func(unlock bool, outs []wallet.LockedOutpoint) error {
			return w.SetLockedOutpoints(unlock, outs)
		},
		WalletIsLockedOutpoint: func(t string, v uint32) bool { return w.IsLockedOutpoint(t, v) },
	}
	lockParam, _ := json.Marshal([]map[string]interface{}{{"txid": txid, "vout": 0}})
	_, code, msg := execLockUnspentWallet(paths, []json.RawMessage{json.RawMessage(`false`), lockParam})
	if code != 0 {
		t.Fatalf("lock: %s", msg)
	}
	unsigned := &wire.Tx{Version: 1, Vin: []wire.TxIn{}, Vout: []wire.TxOut{{Value: 1e8, PkScript: walletPK}}}
	raw, _ := unsigned.Serialize()
	hexJ, _ := json.Marshal(hex.EncodeToString(raw))
	opts, _ := json.Marshal(map[string]interface{}{"changeAddress": w.Address()})
	_, code, msg = execFundRawTransaction("test", paths, nil, nil, nil, []json.RawMessage{hexJ, opts})
	if code != -6 {
		t.Fatalf("want insufficient funds, got %d %s", code, msg)
	}
}
