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

func TestDumpImportWalletDescriptorLine(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	pub1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	pub2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
	desc := "sh(multi(2," + pub1 + "," + pub2 + "))"
	parsed, ok := parseImportDescriptor(desc)
	if !ok {
		t.Fatal("parse desc")
	}
	pkScript, addr, ok := pkScriptAndAddressFromParsedDescriptor(p, parsed)
	if !ok || addr == "" {
		t.Fatal("addr from desc")
	}
	_ = w.AddWatchScript(pkScript)
	_ = w.SetWatchRedeem(pkScript, parsed.redeem)
	_ = w.AddImportedDescriptor(desc, 0, false, false)

	dumpPath := filepath.Join(dir, "dump.txt")
	paths := importDescTestPaths(w, p)
	paths.WalletWatchRedeemScript = w.WatchRedeemScript
	destJ, _ := json.Marshal(dumpPath)
	if _, code, msg := execDumpWallet("testnet", paths, []json.RawMessage{destJ}); code != 0 {
		t.Fatal(msg)
	}
	body, _ := os.ReadFile(dumpPath)
	if !strings.Contains(string(body), "descriptor=1 "+desc) {
		t.Fatalf("dump missing descriptor line: %s", body)
	}
	if strings.Contains(string(body), "redeem=1") {
		t.Fatalf("expected descriptor not redeem line: %s", body)
	}

	dir2 := t.TempDir()
	w2, _ := wallet.LoadOrCreate(filepath.Join(dir2, "w2.json"), p.PubkeyHashAddrID)
	paths2 := importDescTestPaths(w2, p)
	paths2.WalletListDescriptors = paths.WalletListDescriptors
	paths2.WalletWatchRedeemScript = w2.WatchRedeemScript
	paths2.WalletImportSpendKey = func(string) error { return nil }
	paths2.WalletImportPrivKey = func(s string) error {
		return w2.ImportPrivKey(s, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
	}
	paths2.WalletAddress = func() string { return w2.Address() }
	paths2.WalletDefaultAddress = func() string { return w2.Address() }
	if _, code, msg := execImportWallet("testnet", paths2, nil, nil, []json.RawMessage{destJ}); code != 0 {
		t.Fatal(msg)
	}
	if got := w2.WatchRedeemScript(pkScript); len(got) == 0 {
		t.Fatal("redeem not imported via descriptor dump")
	}
	info, code, msg := execGetAddressInfo("testnet", paths2, []json.RawMessage{mustJSON(t, addr)})
	if code != 0 {
		t.Fatal(msg)
	}
	m, _ := info.(map[string]interface{})
	if d, _ := m["desc"].(string); d != desc {
		t.Fatalf("getaddressinfo desc %q want %q", d, desc)
	}
}

func TestGetAddressInfoDescPKH(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	wif, _ := w.WIFExport(p.PrivKeyWIFVersion)
	addr, err := addressFromWIF("testnet", wif)
	if err != nil {
		t.Fatal(err)
	}
	desc := "pkh(" + addr + ")"
	_ = w.AddImportedDescriptor(desc, 0, false, true)
	paths := importDescTestPaths(w, p)
	info, code, msg := execGetAddressInfo("testnet", paths, mustWalletJSONParam(t, addr))
	if code != 0 {
		t.Fatal(msg)
	}
	m, _ := info.(map[string]interface{})
	if d, _ := m["desc"].(string); d != desc {
		t.Fatalf("desc %q want %q", d, desc)
	}
}
