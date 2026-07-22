// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dogego/secp256k1"

	"dogego/chain"
	"dogego/mempool"
	"dogego/primitives"
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

func TestExecImportAddressWatchOnly(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	wpath := filepath.Join(dir, "wallet.json")
	w, err := wallet.LoadOrCreate(wpath, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	watchPK := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
	watchPK[3] = 0x42
	watchPK = append(watchPK, 0x88, 0xac)
	watchAddr := chain.PayToPubKeyHashAddress(watchPK, p.PubkeyHashAddrID)

	paths := &DataPaths{
		WalletAddress:        func() string { return w.Address() },
		WalletImportWatch:    func(script []byte) error { return w.AddWatchScript(script) },
		WalletWatchScripts:   func() [][]byte { return w.WatchScripts() },
		WalletIsWatchAddress: func(addr string) bool {
			return w.IsWatchAddress(addr, p.PubkeyHashAddrID, p.ScriptHashAddrID)
		},
	}
	j := &memJournal{tip: 10, best: "a", gen: "b", count: 11, hdrs: make([][]byte, 11)}
	for i := range j.hdrs {
		j.hdrs[i] = make([]byte, 80)
	}
	addrJ, _ := json.Marshal(watchAddr)
	_, code, msg := execImportAddress("test", paths, j, nil, []json.RawMessage{addrJ})
	if code != 0 {
		t.Fatalf("importaddress: code=%d msg=%s", code, msg)
	}
	if !w.IsWatchAddress(watchAddr, p.PubkeyHashAddrID, p.ScriptHashAddrID) {
		t.Fatalf("watch addr not tracked: %s", watchAddr)
	}
}

func TestWalletUtxoMatchesWatchAndSpend(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	wpath := filepath.Join(dir, "wallet.json")
	w, err := wallet.LoadOrCreate(wpath, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	spendPK := w.P2PKHScript()
	watchPK := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
	watchPK[7] = 0x99
	watchPK = append(watchPK, 0x88, 0xac)
	if err := w.AddWatchScript(watchPK); err != nil {
		t.Fatal(err)
	}
	spendAddr := w.Address()
	watchAddr := chain.PayToPubKeyHashAddress(watchPK, p.PubkeyHashAddrID)

	utxo := store.NewUtxoCache()
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout: []wire.TxOut{
			{Value: 1e8, PkScript: spendPK},
			{Value: 2e8, PkScript: watchPK},
		},
	}
	pb := &wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{coin}}
	if err := utxo.ApplyBlock(pb, 0); err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 5, best: "a", gen: "b", count: 6, hdrs: make([][]byte, 6)}
	for i := range j.hdrs {
		j.hdrs[i] = make([]byte, 80)
	}
	paths := &DataPaths{
		Utxo:               utxo,
		WalletAddress:      func() string { return spendAddr },
		WalletP2PKHScript:  func() []byte { return spendPK },
		WalletWatchScripts: func() [][]byte { return w.WatchScripts() },
	}
	matches, code, msg := walletUtxoMatches(paths, j, nil, "test", 1, 0)
	if code != 0 {
		t.Fatalf("matches: %s", msg)
	}
	if len(matches) != 2 {
		t.Fatalf("want 2 matches got %d", len(matches))
	}
	var spendable, watch int
	for _, m := range matches {
		if m.spendable {
			spendable++
			if m.address != spendAddr {
				t.Fatalf("spend addr %s", m.address)
			}
		} else {
			watch++
			if m.address != watchAddr {
				t.Fatalf("watch addr %s", m.address)
			}
		}
	}
	if spendable != 1 || watch != 1 {
		t.Fatalf("spendable=%d watch=%d", spendable, watch)
	}
}

func TestExecListReceivedByAddressWallet(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	wpath := filepath.Join(dir, "wallet.json")
	w, err := wallet.LoadOrCreate(wpath, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	spendPK := w.P2PKHScript()
	utxo := store.NewUtxoCache()
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5e7, PkScript: spendPK}},
	}
	pb := &wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{coin}}
	if err := utxo.ApplyBlock(pb, 2); err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 3, best: "a", gen: "b", count: 4, hdrs: make([][]byte, 4)}
	for i := range j.hdrs {
		j.hdrs[i] = make([]byte, 80)
	}
	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return w.Address() },
		WalletP2PKHScript: func() []byte { return spendPK },
	}
	p0, _ := json.Marshal(1)
	p1, _ := json.Marshal(false)
	res, code, msg := execListReceivedByAddressWallet("test", paths, j, nil, []json.RawMessage{p0, p1})
	if code != 0 {
		t.Fatalf("listreceived: %s", msg)
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 1 {
		t.Fatalf("result %#v", res)
	}
	ent, ok := arr[0].(map[string]interface{})
	if !ok {
		t.Fatal("entry type")
	}
	if ent["address"] != w.Address() {
		t.Fatalf("address %#v", ent["address"])
	}
	if ent["amount"].(float64) != 0.5 {
		t.Fatalf("amount %#v", ent["amount"])
	}
	if wo, ok := ent["iswatchonly"].(bool); !ok || wo {
		t.Fatalf("iswatchonly %#v", ent["iswatchonly"])
	}
}

func TestImportWatchScriptArgHex(t *testing.T) {
	pkHex := "76a914" + hex.EncodeToString(make([]byte, 20)) + "88ac"
	pk, code, msg := importWatchScriptArg("test", pkHex, false)
	if code != 0 || len(pk) == 0 {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
}

func TestExecBackupWallet(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "wallet.json")
	if err := os.WriteFile(src, []byte(`{"privkey_hex":"00"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "backup", "wallet.bak")
	paths := &DataPaths{WalletPath: func() string { return src }}
	destJ, _ := json.Marshal(dest)
	_, code, msg := execBackupWallet(paths, []json.RawMessage{destJ})
	if code != 0 {
		t.Fatalf("backupwallet: %s", msg)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "privkey_hex") {
		t.Fatalf("backup content %q", b)
	}
}

func TestWalletMempoolNetKoinu(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	spendPK := w.P2PKHScript()
	utxo := store.NewUtxoCache()
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 10_000_000_000, PkScript: spendPK}},
	}
	pb := &wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{coin}}
	if err := utxo.ApplyBlock(pb, 0); err != nil {
		t.Fatal(err)
	}
	pool := mempool.New(100)
	recv := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 250_000_000, PkScript: spendPK}},
	}
	raw, _ := recv.Serialize()
	_ = pool.Add(raw)
	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return w.Address() },
		WalletP2PKHScript: func() []byte { return spendPK },
	}
	net := walletMempoolNetKoinu("test", paths, pool)
	if net != 250_000_000 {
		t.Fatalf("mempool net %d", net)
	}
}

func TestExecImportPubKeyWatch(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	sk := make([]byte, 32)
	sk[31] = 9
	priv, _ := secp256k1.PrivKeyFromBytes(sk)
	pubHex := hex.EncodeToString(priv.PubKey().SerializeCompressed())
	paths := &DataPaths{
		WalletAddress:     func() string { return w.Address() },
		WalletImportWatch: func(script []byte) error { return w.AddWatchScript(script) },
	}
	pubJ, _ := json.Marshal(pubHex)
	_, code, msg := execImportPubKey("test", paths, nil, nil, []json.RawMessage{pubJ})
	if code != 0 {
		t.Fatalf("importpubkey: %s", msg)
	}
	if len(w.WatchScripts()) != 1 {
		t.Fatalf("watch scripts %d", len(w.WatchScripts()))
	}
}

func TestExecDumpWallet(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	wif, err := w.WIFExport(p.PrivKeyWIFVersion)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "dump.txt")
	paths := &DataPaths{
		BaseDataDir:   dir,
		ChainDataDir:  dir,
		WalletPath:    func() string { return w.Path() },
		WalletAddress: func() string { return w.Address() },
		WalletWIF:     func() string { return wif },
	}
	destJ, _ := json.Marshal(dest)
	_, code, msg := execDumpWallet("test", paths, []json.RawMessage{destJ})
	if code != 0 {
		t.Fatalf("dumpwallet: %s", msg)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), wif) || !strings.Contains(string(b), w.Address()) {
		t.Fatalf("dump %q", b)
	}
}

func TestExecListAccountsWallet(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	spendPK := w.P2PKHScript()
	utxo := store.NewUtxoCache()
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 3e7, PkScript: spendPK}},
	}
	pb := &wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{coin}}
	_ = utxo.ApplyBlock(pb, 0)
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return w.Address() },
		WalletP2PKHScript: func() []byte { return spendPK },
	}
	res, code, msg := execListAccountsWallet("test", paths, j, nil, nil)
	if code != 0 {
		t.Fatalf("listaccounts: %s", msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok || m[""].(float64) != 0.3 {
		t.Fatalf("result %#v", res)
	}
}

func TestWalletDiskWatchPersist(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	wpath := filepath.Join(dir, "wallet.json")
	w, err := wallet.LoadOrCreate(wpath, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	pk := []byte{0x76, 0xa9, 0x14}
	pk = append(pk, make([]byte, 20)...)
	pk = append(pk, 0x88, 0xac)
	if err := w.AddWatchScript(pk); err != nil {
		t.Fatal(err)
	}
	w2, err := wallet.LoadOrCreate(wpath, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if !w2.HasWatchScript(pk) {
		t.Fatal("watch script not persisted")
	}
	b, err := os.ReadFile(wpath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "watch_scripts") {
		t.Fatalf("wallet file missing watch_scripts: %s", b)
	}
}
