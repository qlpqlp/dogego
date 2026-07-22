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
	"dogego/wallet"
)

func TestDumpImportWalletWatchRedeem(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	k, _ := secp256k1.NewPrivateKey()
	pub := hex.EncodeToString(k.PubKey().SerializeCompressed())
	nReq, _ := json.Marshal(1)
	keys, _ := json.Marshal([]string{pub})
	msRes, code, msg := execCreateMultisig("test", []json.RawMessage{nReq, keys})
	if code != 0 {
		t.Fatal(msg)
	}
	redeem, _ := hex.DecodeString(msRes["redeemScript"].(string))
	h := scriptHash160(redeem)
	p2sh := chain.P2SHScriptFromScriptHash(h)
	_ = w.AddWatchScript(p2sh)
	_ = w.SetWatchRedeem(p2sh, redeem)

	wif, _ := w.WIFExport(p.PrivKeyWIFVersion)
	dumpPath := filepath.Join(dir, "dump.txt")
	paths := mergePathsDataDir(&DataPaths{
		WalletPath:              func() string { return w.Path() },
		WalletAddress:           func() string { return w.Address() },
		WalletWIF:               func() string { return wif },
		WalletWatchScripts:      func() [][]byte { return w.WatchScripts() },
		WalletWatchRedeemScript: func(spk []byte) []byte { return w.WatchRedeemScript(spk) },
	}, dir)
	destJ, _ := json.Marshal(dumpPath)
	if _, code, msg := execDumpWallet("test", paths, []json.RawMessage{destJ}); code != 0 {
		t.Fatal(msg)
	}
	body, _ := os.ReadFile(dumpPath)
	if !strings.Contains(string(body), "redeem=1") && !strings.Contains(string(body), "descriptor=1") {
		t.Fatalf("dump missing redeem or descriptor line: %s", body)
	}

	dir2 := t.TempDir()
	w2, _ := wallet.LoadOrCreate(filepath.Join(dir2, "w2.json"), p.PubkeyHashAddrID)
	paths2 := &DataPaths{
		WalletAddress:         func() string { return w2.Address() },
		WalletImportWatch:     func(spk []byte) error { return w2.AddWatchScript(spk) },
		WalletSetWatchRedeem:  func(spk, r []byte) error { return w2.SetWatchRedeem(spk, r) },
		WalletWatchScripts:    func() [][]byte { return w2.WatchScripts() },
		WalletWatchRedeemScript: func(spk []byte) []byte { return w2.WatchRedeemScript(spk) },
		WalletImportPrivKey:   func(w string) error { return w2.ImportPrivKey(w, p.PrivKeyWIFVersion, p.PubkeyHashAddrID) },
		WalletImportSpendKey:  func(w string) error { return w2.ImportSpendPrivKey(w, p.PrivKeyWIFVersion, p.PubkeyHashAddrID) },
	}
	if _, code, msg := execImportWallet("test", paths2, nil, nil, []json.RawMessage{destJ}); code != 0 {
		t.Fatal(msg)
	}
	got := w2.WatchRedeemScript(p2sh)
	if len(got) == 0 || string(got) != string(redeem) {
		t.Fatalf("redeem not round-tripped: %x vs %x", got, redeem)
	}
}
