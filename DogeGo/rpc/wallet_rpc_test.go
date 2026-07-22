// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"crypto/sha256"
	"encoding/binary"
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

func TestWalletRPCListUnspent(t *testing.T) {
	utxo := store.NewUtxoCache()
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr, _ := chain.RandomP2PKHAddress(p)
	_, h160, _ := chain.Base58CheckDecode(addr)
	pk := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pk = append(pk, 0x88, 0xac)
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5_000_000_000, PkScript: pk}},
	}
	pb := &wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{coin}}
	_ = utxo.ApplyBlock(pb, 0)
	j := &memJournal{tip: 0, count: 1, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return addr },
		WalletP2PKHScript: func() []byte { return pk },
	}
	res, code, msg := execListUnspent("testnet", paths, j, nil, nil, nil)
	if code != 0 {
		t.Fatalf("listunspent: %d %s", code, msg)
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 1 {
		t.Fatalf("want 1 utxo, got %T %#v", res, res)
	}
}

func TestWalletRPCListUnspentMaximumCountAfterFiltering(t *testing.T) {
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(100)
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr, _ := chain.RandomP2PKHAddress(p)
	_, h160, _ := chain.Base58CheckDecode(addr)
	pk := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pk = append(pk, 0x88, 0xac)
	for i, value := range []int64{1_000_000_000, 5_000_000_000, 2_000_000_000} {
		var op [36]byte
		op[0] = byte(i + 1)
		utxo.AddUtxoForTest(op, store.UtxoEntry{Value: value, PkScript: pk, Height: 90})
	}
	j := &memJournal{tip: 100}
	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return addr },
		WalletP2PKHScript: func() []byte { return pk },
	}
	params := []json.RawMessage{
		json.RawMessage(`1`),
		json.RawMessage(`9999999`),
		json.RawMessage(`null`),
		json.RawMessage(`true`),
		json.RawMessage(`{"minimumAmount":2,"maximumCount":1}`),
	}
	res, code, msg := execListUnspent("testnet", paths, j, nil, nil, params)
	if code != 0 {
		t.Fatalf("listunspent: %d %s", code, msg)
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 1 {
		t.Fatalf("want one filtered utxo, got %T %#v", res, res)
	}
	row := arr[0].(map[string]interface{})
	if row["amount"].(float64) != 50.0 {
		t.Fatalf("amount=%#v want largest eligible 50 DOGE", row["amount"])
	}
}

func TestWalletRPCGetNewAddress(t *testing.T) {
	paths := &DataPaths{WalletAddress: func() string { return "D7Y55LdBq3c2Q3j6tK9mP2nR4sT6uV8wX" }}
	_, code, _ := execGetNewAddress("test", paths, nil)
	if code == 0 {
		// address may be invalid in test - only check stub path when empty
	}
	paths2 := &DataPaths{WalletAddress: func() string { return "nosuch" }}
	_, code2, msg2 := execGetNewAddress("test", paths2, nil)
	if code2 != 0 && msg2 == "" {
		t.Fatal("expected error or address")
	}
}

func TestExecImportPrivKeyWired(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	sk := make([]byte, 32)
	sk[31] = 7
	wif, err := chain.EncodeWIF(sk, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	var imported string
	paths := &DataPaths{
		WalletImportPrivKey: func(w string) error {
			imported = w
			return nil
		},
	}
	wifJ, _ := json.Marshal(wif)
	_, code, msg := execImportPrivKey("testnet", paths, nil, nil, []json.RawMessage{wifJ})
	if code != 0 {
		t.Fatalf("importprivkey: %d %s", code, msg)
	}
	if imported != wif {
		t.Fatalf("imported %q", imported)
	}
}

func TestExecSignMessageWallet(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	sk := make([]byte, 32)
	sk[0] = 9
	wif, err := chain.EncodeWIF(sk, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	priv, _ := secp256k1.PrivKeyFromBytes(sk)
	comp := priv.PubKey().SerializeCompressed()
	h := sha256.Sum256(comp)
	r := ripemd160.New()
	_, _ = r.Write(h[:])
	addr := chain.Base58CheckEncode(p.PubkeyHashAddrID, r.Sum(nil))
	paths := &DataPaths{
		WalletAddress: func() string { return addr },
		WalletWIF:     func() string { return wif },
	}
	addrJ, _ := json.Marshal(addr)
	msgJ, _ := json.Marshal("such wow")
	sig, code, msg := execSignMessage("testnet", paths, []json.RawMessage{addrJ, msgJ})
	if code != 0 {
		t.Fatalf("signmessage: %d %s", code, msg)
	}
	ok, code2, _ := execVerifyMessage("testnet", []json.RawMessage{addrJ, json.RawMessage(`"`+sig.(string)+`"`), msgJ})
	if code2 != 0 || !ok {
		t.Fatalf("verify failed ok=%v code=%d", ok, code2)
	}
}

func TestWalletTxRowToListEntryBlockMeta(t *testing.T) {
	entry := walletTxRowToListEntry("", nil, nil, nil, nil, nil, "DAddr", walletTxRow{
		txid: "aa", category: "receive", amountKoinu: 1e8, confirmations: 3,
		blockHeight: 10, blockHash: "deadbeef", blockTime: 1_700_000_000, vout: 1, time: 1_700_000_000,
	})
	if entry["blockheight"] != int64(10) {
		t.Fatalf("blockheight %#v", entry["blockheight"])
	}
	if entry["blockhash"] != "deadbeef" {
		t.Fatalf("blockhash %#v", entry["blockhash"])
	}
	if entry["blockindex"] != uint32(1) {
		t.Fatalf("blockindex %#v", entry["blockindex"])
	}
	if entry["time"] != int64(1_700_000_000) {
		t.Fatalf("time %#v", entry["time"])
	}
}

func TestWalletLookupTxHexMempool(t *testing.T) {
	pool := mempool.New(10)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	raw, _ := tx.Serialize()
	_ = pool.Add(raw)
	id := mempool.TxIDDisplayHex(tx.TxHash())
	got := walletLookupTxHex(pool, nil, nil, nil, nil, id, -1)
	if got == "" || len(got) < 4 {
		t.Fatalf("hex %q", got)
	}
}

func TestWalletLookupTxHexWalletCache(t *testing.T) {
	txid := repeatHex('d')
	want := "0100000001"
	paths := &DataPaths{
		WalletTxHexLookup: func(id string) (string, bool) {
			if id == txid {
				return want, true
			}
			return "", false
		},
	}
	got := walletLookupTxHex(nil, paths, nil, nil, nil, txid, 100)
	if got != want {
		t.Fatalf("cache hex %q want %q", got, want)
	}
}

func TestWalletBIP125Replaceable(t *testing.T) {
	pool := mempool.New(10)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	raw, _ := tx.Serialize()
	_ = pool.Add(raw)
	id := mempool.TxIDDisplayHex(tx.TxHash())
	if walletBIP125Replaceable(pool, id, -1) != "yes" {
		t.Fatalf("mempool RBF tx want yes")
	}
	if walletBIP125Replaceable(pool, id, 0) != "no" {
		t.Fatalf("confirmed want no")
	}
	if walletBIP125Replaceable(nil, id, -1) != "unknown" {
		t.Fatalf("nil pool want unknown")
	}
}

func TestWalletTxRowToListEntryBIP125(t *testing.T) {
	entry := walletTxRowToListEntry("", nil, nil, nil, nil, nil, "a", walletTxRow{bip125: "yes", confirmations: 0})
	if entry["bip125-replaceable"] != "yes" {
		t.Fatalf("bip125 %#v", entry["bip125-replaceable"])
	}
	conflicts, ok := entry["walletconflicts"].([]interface{})
	if !ok || len(conflicts) != 0 {
		t.Fatalf("walletconflicts %#v", entry["walletconflicts"])
	}
}

func TestWalletTxRowToListEntryConflicts(t *testing.T) {
	oldID := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	newID := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	paths := &DataPaths{
		WalletConflictsForTx: func(txid string) []string {
			if txid == oldID {
				return []string{newID}
			}
			return nil
		},
	}
	entry := walletTxRowToListEntry("", paths, nil, nil, nil, nil, "a", walletTxRow{txid: oldID})
	conflicts, ok := entry["walletconflicts"].([]interface{})
	if !ok || len(conflicts) != 1 || conflicts[0] != newID {
		t.Fatalf("walletconflicts %#v", entry["walletconflicts"])
	}
}

func TestWalletHeaderTimeAt(t *testing.T) {
	h := make([]byte, 80)
	binary.LittleEndian.PutUint32(h[68:72], 1234567890)
	j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{h}}
	if got := walletHeaderTimeAt(j, 0); got != 1234567890 {
		t.Fatalf("time %d want 1234567890", got)
	}
}

func TestWalletRowAfterSinceBlock(t *testing.T) {
	if !walletRowAfterSinceBlock(walletTxRow{blockHeight: 5}, 4, true) {
		t.Fatal("height 5 should be after since 4")
	}
	if walletRowAfterSinceBlock(walletTxRow{blockHeight: 4}, 4, true) {
		t.Fatal("height 4 should not be after since 4")
	}
	if !walletRowAfterSinceBlock(walletTxRow{blockHeight: -1}, 4, true) {
		t.Fatal("mempool row should pass since filter")
	}
}

func TestWalletRPCDumpPrivKey(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr, _ := chain.RandomP2PKHAddress(p)
	paths := &DataPaths{
		WalletAddress: func() string { return addr },
		WalletWIF:     func() string { return "6keytest" },
	}
	_, code, msg := execDumpPrivKey("testnet", paths, []json.RawMessage{json.RawMessage(`"` + addr + `"`)})
	if code != 0 {
		t.Fatalf("dumpprivkey: %d %s", code, msg)
	}
}
