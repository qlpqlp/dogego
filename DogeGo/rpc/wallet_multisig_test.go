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

	"dogego/secp256k1"

	"dogego/chain"
	"dogego/primitives"
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

func TestAddMultisigAddressWalletWatch(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	keysJ, _ := json.Marshal([]string{
		hexEncode(k1.PubKey().SerializeCompressed()),
		hexEncode(k2.PubKey().SerializeCompressed()),
	})
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		WalletAddress:     func() string { return w.Address() },
		WalletImportWatch: func(script []byte) error { return w.AddWatchScript(script) },
		WalletWatchScripts: func() [][]byte { return w.WatchScripts() },
		WalletIsWatchAddress: func(addr string) bool {
			return w.IsWatchAddress(addr, p.PubkeyHashAddrID, p.ScriptHashAddrID)
		},
	}
	nJ, _ := json.Marshal(2)
	addr, code, msg := execAddMultisigAddressWallet("test", paths, []json.RawMessage{nJ, keysJ})
	if code != 0 {
		t.Fatalf("addmultisig: %s", msg)
	}
	msAddr, ok := addr.(string)
	if !ok || msAddr == "" {
		t.Fatalf("addr %#v", addr)
	}
	if !w.IsWatchAddress(msAddr, p.PubkeyHashAddrID, p.ScriptHashAddrID) {
		t.Fatal("multisig not watch")
	}
}

func TestAddMultisigAddressTracksUtxo(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	keysJ, _ := json.Marshal([]string{
		hexEncode(k1.PubKey().SerializeCompressed()),
		hexEncode(k2.PubKey().SerializeCompressed()),
	})
	nJ, _ := json.Marshal(2)
	m, _, _ := execCreateMultisig("test", []json.RawMessage{nJ, keysJ})
	msAddr := m["address"].(string)
	redeemHex := m["redeemScript"].(string)
	redeem, _ := hex.DecodeString(redeemHex)
	h := scriptHash160(redeem)
	p2sh := chain.P2SHScriptFromScriptHash(h)

	dir := t.TempDir()
	w, _ := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	_ = w.AddWatchScript(p2sh)
	utxo := store.NewUtxoCache()
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 2e8, PkScript: p2sh}},
	}
	pb := &wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{coin}}
	_ = utxo.ApplyBlock(pb, 0)
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return w.Address() },
		WalletP2PKHScript: func() []byte { return w.P2PKHScript() },
		WalletWatchScripts: func() [][]byte { return w.WatchScripts() },
	}
	matches, code, msg := walletUtxoMatches(paths, j, nil, "test", 1, 0)
	if code != 0 {
		t.Fatalf("matches: %s", msg)
	}
	if len(matches) != 1 || matches[0].address != msAddr {
		t.Fatalf("matches %#v addr %s", matches, msAddr)
	}
}

func TestExecKeypoolRefillWallet(t *testing.T) {
	paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
	res, code, msg := execKeypoolRefillWallet(paths, nil)
	if code != 0 || res != true {
		t.Fatalf("keypoolrefill %v %d %s", res, code, msg)
	}
}

func hexEncode(b []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexdigits[v>>4]
		out[i*2+1] = hexdigits[v&0x0f]
	}
	return string(out)
}
