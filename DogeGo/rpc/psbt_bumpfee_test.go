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
	"dogego/mempool"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

func TestExecPsbtBumpFeeWallet(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	utxo := store.NewUtxoCache()
	_, h160, _ := chain.Base58CheckDecode(mustTestWalletAddr(t, p))
	walletPK := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	walletPK = append(walletPK, 0x88, 0xac)
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 10_000_000_000, PkScript: walletPK}},
	}
	pb := &wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{coin}}
	if err := utxo.ApplyBlock(pb, 0); err != nil {
		t.Fatal(err)
	}
	coinID := txidToRPC(coin.TxHash())
	prev, _ := decodeRPCPrevHashHex(coinID)
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: prev,
			PrevIdx:  0,
			Sequence: wire.MaxBIP125RBFSequence,
		}},
		Vout: []wire.TxOut{
			{Value: 1_000_000_000, PkScript: walletPK},
			{Value: 8_000_000_000, PkScript: walletPK},
		},
	}
	spendRaw, err := spend.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	pool := mempool.New(100)
	_ = pool.Add(spendRaw)
	spendID := txidToRPC(spend.TxHash())
	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return mustTestWalletAddr(t, p) },
		WalletWIF:         func() string { return mustTestWalletWIF(t, p) },
		WalletP2PKHScript: func() []byte { return walletPK },
	}
	res, code, msg := execPsbtBumpFee("testnet", paths, pool, nil, nil, []json.RawMessage{
		json.RawMessage(`"` + spendID + `"`),
		json.RawMessage(`{}`),
	})
	if code != 0 {
		t.Fatalf("psbtbumpfee: %d %s", code, msg)
	}
	m := res.(map[string]interface{})
	if m["psbt"] == "" || m["psbt"] == nil {
		t.Fatalf("missing psbt %#v", m)
	}
	newFee, _ := m["fee"].(float64)
	origFee, _ := m["origfee"].(float64)
	if newFee <= origFee {
		t.Fatalf("fee should increase: orig=%v new=%v", origFee, newFee)
	}
}

func TestExecSimulateRawTransactionSpend(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	utxo := store.NewUtxoCache()
	_, h160, _ := chain.Base58CheckDecode(mustTestWalletAddr(t, p))
	walletPK := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	walletPK = append(walletPK, 0x88, 0xac)
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5_000_000_000, PkScript: walletPK}},
	}
	if err := utxo.ApplyBlock(&wire.ParsedBlock{
		Header: primitives.BlockHeader{Version: 1, Timestamp: 1},
		Txs:    []*wire.Tx{coin},
	}, 0); err != nil {
		t.Fatal(err)
	}
	coinID := txidToRPC(coin.TxHash())
	prev, _ := decodeRPCPrevHashHex(coinID)
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 4_000_000_000, PkScript: []byte{0x51}}},
	}
	spendRaw, _ := spend.Serialize()
	pool := mempool.New(100)
	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return mustTestWalletAddr(t, p) },
		WalletP2PKHScript: func() []byte { return walletPK },
	}
	arr, _ := json.Marshal([]string{hexEncode(spendRaw)})
	delta, code, msg := execSimulateRawTransaction("testnet", paths, pool, nil, nil, []json.RawMessage{arr})
	if code != 0 {
		t.Fatalf("simulate: %d %s", code, msg)
	}
	d, ok := delta.(float64)
	if !ok || d >= 0 {
		t.Fatalf("expected negative balance change, got %v", delta)
	}
}

func TestExecImportMempoolRelativeEmpty(t *testing.T) {
	dir := t.TempDir()
	path := mempool.PersistPath(dir)
	if err := mempool.SavePersisted(path, nil, nil); err != nil {
		t.Fatal(err)
	}
	pool := mempool.New(10)
	paths := &DataPaths{ChainDataDir: dir}
	nameJ, _ := json.Marshal(mempool.PersistFileName)
	res, code, msg := execImportMempool(pool, paths, nil, nil, nil, chain.MainnetDogecoin, []json.RawMessage{nameJ})
	if code != 0 {
		t.Fatalf("importmempool: %d %s", code, msg)
	}
	m := res.(map[string]interface{})
	if m["imported"] != 0 && m["imported"] != float64(0) {
		t.Fatalf("expected 0 imported %#v", m)
	}
}

func TestExecImportMempoolMissingFile(t *testing.T) {
	dir := t.TempDir()
	paths := &DataPaths{ChainDataDir: dir}
	abs := filepath.Join(dir, "missing.json")
	pathJ, _ := json.Marshal(abs)
	_, code, _ := execImportMempool(mempool.New(10), paths, nil, nil, nil, chain.MainnetDogecoin, []json.RawMessage{pathJ})
	if code != -8 {
		t.Fatalf("want -8 for missing file, got %d", code)
	}
}

func hexEncodeBytes(b []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexdigits[v>>4]
		out[i*2+1] = hexdigits[v&0x0f]
	}
	return string(out)
}
