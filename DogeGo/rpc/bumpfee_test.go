// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"crypto/sha256"
	"encoding/json"
	"testing"

	"dogego/secp256k1"
	"golang.org/x/crypto/ripemd160"

	"dogego/chain"
	"dogego/mempool"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

func TestExecBumpFeeRequiresRawTx(t *testing.T) {
	pool := mempool.New(100)
	parentHash := [32]byte{7}
	old := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: []byte{0x51}}},
	}
	oldRaw, _ := old.Serialize()
	oldID := txidToRPC(old.TxHash())
	_ = pool.Add(oldRaw)
	_, code, msg := execBumpFee("main", pool, nil, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`"` + oldID + `"`),
	}, nil, chain.MainnetDogecoin)
	if code != -8 || msg == "" {
		t.Fatalf("want -8 rawtx required, got %d %q", code, msg)
	}
}

func TestExecBumpFeeNotInMempool(t *testing.T) {
	pool := mempool.New(100)
	_, code, _ := execBumpFee("main", pool, nil, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`"0000000000000000000000000000000000000000000000000000000000000001"`),
		json.RawMessage(`{"rawtx":"01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"}`),
	}, nil, chain.MainnetDogecoin)
	if code != -5 {
		t.Fatalf("want -5 for missing mempool tx, got %d", code)
	}
}

func TestExecBumpFeeWalletAuto(t *testing.T) {
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
	destPK := walletPK
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: prev,
			PrevIdx:  0,
			Sequence: wire.MaxBIP125RBFSequence,
		}},
		Vout: []wire.TxOut{
			{Value: 1_000_000_000, PkScript: destPK},
			{Value: 8_000_000_000, PkScript: walletPK},
		},
	}
	spendRaw, err := spend.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	pool := mempool.New(100)
	if err := pool.Add(spendRaw); err != nil {
		t.Fatal(err)
	}
	spendID := txidToRPC(spend.TxHash())
	wif := mustTestWalletWIF(t, p)
	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return mustTestWalletAddr(t, p) },
		WalletWIF:         func() string { return wif },
		WalletP2PKHScript: func() []byte { return walletPK },
	}
	res, code, msg := execBumpFee("testnet", pool, nil, nil, nil, paths, []json.RawMessage{
		json.RawMessage(`"` + spendID + `"`),
		json.RawMessage(`{}`),
	}, nil, chain.RebootTestnet)
	if code != 0 {
		t.Fatalf("bumpfee: %d %s", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("result %T", res)
	}
	if m["txid"] == spendID {
		t.Fatal("expected new txid after bump")
	}
	newFee, _ := m["fee"].(float64)
	origFee, _ := m["origfee"].(float64)
	if newFee <= origFee {
		t.Fatalf("fee should increase: orig=%v new=%v", origFee, newFee)
	}
}

func mustTestWalletAddr(t *testing.T, p chain.Params) string {
	t.Helper()
	sk := make([]byte, 32)
	sk[0] = 11
	wif, err := chain.EncodeWIF(sk, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := chain.DecodeWIF(wif, p.PrivKeyWIFVersion)
	if err != nil {
		t.Fatal(err)
	}
	priv, _ := secp256k1.PrivKeyFromBytes(key)
	comp := priv.PubKey().SerializeCompressed()
	h := sha256.Sum256(comp)
	r := ripemd160.New()
	_, _ = r.Write(h[:])
	return chain.Base58CheckEncode(p.PubkeyHashAddrID, r.Sum(nil))
}

func mustTestWalletWIF(t *testing.T, p chain.Params) string {
	t.Helper()
	sk := make([]byte, 32)
	sk[0] = 11
	wif, err := chain.EncodeWIF(sk, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	return wif
}
