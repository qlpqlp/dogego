// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/wallet"
)

func TestParseDumpWalletLine(t *testing.T) {
	wif, scr, red, desc, ok := parseDumpWalletLine("# comment")
	if ok {
		t.Fatal("comment")
	}
	wif, scr, red, desc, ok = parseDumpWalletLine("1700000000,5HueCG... label= # addr=D...")
	if !ok || wif == "" || scr != "" || red != "" || desc != "" {
		t.Fatalf("wif line %#v %#v %#v %#v", wif, scr, red, desc)
	}
	_, scr, red, desc, ok = parseDumpWalletLine("script=1 76a9140102030405060708090a0b0c0d0e0f10111213141516171819192021222324252688ac label=")
	if !ok || scr == "" || red != "" || desc != "" {
		t.Fatalf("script line scr=%q ok=%v", scr, ok)
	}
	_, scr, red, desc, ok = parseDumpWalletLine("redeem=1 a9140102030405060708090a0b0c0d0e0f10111213141516171887 52ae label=")
	if !ok || scr == "" || red == "" || desc != "" {
		t.Fatalf("redeem line scr=%q red=%q", scr, red)
	}
	_, scr, red, desc, ok = parseDumpWalletLine("descriptor=1 sh(multi(1,02abc)) label= # addr=9xxx")
	if !ok || desc != "sh(multi(1,02abc))" || scr != "" {
		t.Fatalf("descriptor line desc=%q ok=%v", desc, ok)
	}
}

func TestImportWalletFromDump(t *testing.T) {
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
	dumpPath := filepath.Join(dir, "dump.txt")
	paths := mergePathsDataDir(&DataPaths{
		WalletPath:    func() string { return w.Path() },
		WalletAddress: func() string { return w.Address() },
		WalletWIF:     func() string { return wif },
	}, dir)
	_, code, msg := execDumpWallet("test", paths, mustWalletJSONParam(t, dumpPath))
	if code != 0 {
		t.Fatalf("dump: %s", msg)
	}
	dir2 := t.TempDir()
	w2, err := wallet.LoadOrCreate(filepath.Join(dir2, "wallet2.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	var importedWIF string
	paths2 := &DataPaths{
		WalletAddress: func() string { return w2.Address() },
		WalletImportSpendKey: func(w string) error {
			importedWIF = w
			return w2.ImportSpendPrivKey(w, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
		},
		WalletImportPrivKey: func(w string) error {
			return w2.ImportPrivKey(w, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
		},
		WalletImportWatch: func(script []byte) error { return w2.AddWatchScript(script) },
		WalletWatchScripts: func() [][]byte { return w2.WatchScripts() },
	}
	_, code, msg = execImportWallet("test", paths2, nil, nil, mustWalletJSONParam(t, dumpPath))
	if code != 0 {
		t.Fatalf("import: %s", msg)
	}
	if importedWIF != wif {
		t.Fatalf("wif %q vs %q", importedWIF, wif)
	}
	exported, err := w2.WIFExport(p.PrivKeyWIFVersion)
	if err != nil || exported != wif {
		t.Fatalf("export %q err %v", exported, err)
	}
}

func TestImportWalletMultiKeyDump(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir1 := t.TempDir()
	w1, err := wallet.LoadOrCreate(filepath.Join(dir1, "w1.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	wif1, err := w1.WIFExport(p.PrivKeyWIFVersion)
	if err != nil {
		t.Fatal(err)
	}
	dir2 := t.TempDir()
	w2, err := wallet.LoadOrCreate(filepath.Join(dir2, "w2.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	wif2, err := w2.WIFExport(p.PrivKeyWIFVersion)
	if err != nil {
		t.Fatal(err)
	}
	if wif1 == wif2 {
		t.Fatal("expected distinct wallet keys")
	}
	dumpPath := filepath.Join(t.TempDir(), "multi.dump")
	dump := "# multi key dump\n1700000001," + wif1 + " label=first\n1700000002," + wif2 + " label=second\n"
	if err := os.WriteFile(dumpPath, []byte(dump), 0o600); err != nil {
		t.Fatal(err)
	}
	dir3 := t.TempDir()
	w3, err := wallet.LoadOrCreate(filepath.Join(dir3, "w3.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		WalletAddress: func() string { return w3.Address() },
		WalletImportSpendKey: func(w string) error {
			return w3.ImportSpendPrivKey(w, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
		},
		WalletImportPrivKey: func(w string) error {
			return w3.ImportPrivKey(w, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
		},
		WalletImportWatch: func(script []byte) error { return w3.AddWatchScript(script) },
		WalletWatchScripts: func() [][]byte { return w3.WatchScripts() },
	}
	_, code, msg := execImportWallet("test", paths, nil, nil, mustWalletJSONParam(t, dumpPath))
	if code != 0 {
		t.Fatalf("import: %s", msg)
	}
	all, err := w3.AllWIFs(p.PrivKeyWIFVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("wifs=%v", all)
	}
	seen := map[string]bool{all[0]: true, all[1]: true}
	if !seen[wif1] || !seen[wif2] {
		t.Fatalf("missing wif1/wif2 in %v", all)
	}
}

func TestImportWalletCallsKeypoolRefill(t *testing.T) {
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
	dumpPath := filepath.Join(dir, "one.dump")
	if err := os.WriteFile(dumpPath, []byte("1700000001,"+wif+" label=\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var refilled bool
	paths := &DataPaths{
		WalletAddress: func() string { return w.Address() },
		WalletImportSpendKey: func(wif string) error {
			return w.ImportSpendPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
		},
		WalletImportPrivKey: func(wif string) error {
			return w.ImportPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
		},
		WalletImportWatch:    func(script []byte) error { return w.AddWatchScript(script) },
		WalletWatchScripts:   func() [][]byte { return w.WatchScripts() },
		WalletKeypoolRefill: func(n int) error {
			refilled = true
			if n != 0 {
				t.Fatalf("newsize=%d want 0", n)
			}
			return nil
		},
	}
	_, code, msg := execImportWallet("test", paths, nil, nil, mustWalletJSONParam(t, dumpPath))
	if code != 0 {
		t.Fatalf("import: %s", msg)
	}
	if !refilled {
		t.Fatal("expected keypool refill after spend-key import")
	}
}

func TestExecSetTxFeeWallet(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		WalletAddress:     func() string { return w.Address() },
		WalletPayTxFee:    func() float64 { return w.PayTxFee() },
		WalletSetPayTxFee: func(f float64) error { return w.SetPayTxFee(f) },
	}
	_, code, msg := execSetTxFee(paths, mustWalletJSONParam(t, 0.01))
	if code != 0 {
		t.Fatalf("settxfee: %s", msg)
	}
	if w.PayTxFee() != 0.01 {
		t.Fatalf("fee %v", w.PayTxFee())
	}
	if rpcWalletPayTxFeeKoinuPerKB(paths) != 1_000_000 {
		t.Fatalf("koinu %d", rpcWalletPayTxFeeKoinuPerKB(paths))
	}
}

func TestExecRescanWallet(t *testing.T) {
	paths := &DataPaths{WalletAddress: func() string { return "D7Test" }}
	j := &memJournal{tip: 5, best: "a", gen: "b", count: 6, hdrs: make([][]byte, 6)}
	for i := range j.hdrs {
		j.hdrs[i] = make([]byte, 80)
	}
	res, code, msg := execRescanWallet(paths, j, nil, mustWalletJSONParam(t, 0))
	if code != 0 {
		t.Fatalf("rescan: %s", msg)
	}
	if res != nil {
		t.Fatalf("result %#v", res)
	}
}

func mustWalletJSONParam(t *testing.T, v interface{}) []json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return []json.RawMessage{b}
}
