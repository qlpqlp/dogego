// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/wallet"
	"dogego/wallet/corewallet"
)

func TestIsBerkeleyWalletDat(t *testing.T) {
	dir := t.TempDir()
	bdb := filepath.Join(dir, "wallet.dat")
	page := make([]byte, 512)
	binary.LittleEndian.PutUint32(page[12:16], 0x00053162)
	binary.LittleEndian.PutUint32(page[20:24], 512)
	page[28] = 9 // btree meta
	if err := os.WriteFile(bdb, page, 0o600); err != nil {
		t.Fatal(err)
	}
	if !isBerkeleyWalletDat(bdb) {
		t.Fatal("expected Berkeley DB magic")
	}
	txt := filepath.Join(dir, "dump.txt")
	if err := os.WriteFile(txt, []byte("# wallet dump\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if isBerkeleyWalletDat(txt) {
		t.Fatal("text dump should not match BDB magic")
	}
}

func TestIsTextWalletDump(t *testing.T) {
	dir := t.TempDir()
	dump := filepath.Join(dir, "dump.txt")
	if err := os.WriteFile(dump, []byte("# Dogecoin Core wallet dump\n1700000000,5HueCG... label=\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !isTextWalletDump(dump) {
		t.Fatal("comment-prefixed dump should be recognized")
	}
}

func TestExecImportWalletDatBerkeleyWithoutCoreRPC(t *testing.T) {
	dir := t.TempDir()
	bdb := filepath.Join(dir, "wallet.dat")
	page := make([]byte, 512)
	binary.LittleEndian.PutUint32(page[12:16], 0x00053162)
	binary.LittleEndian.PutUint32(page[20:24], 512)
	page[28] = 9
	if err := os.WriteFile(bdb, page, 0o600); err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		ChainDataDir:         dir,
		WalletImportSpendKey: func(string) error { return nil },
	}
	_, code, msg := execImportWalletDat("test", paths, nil, nil, mustWalletJSONParam(t, bdb))
	if code != -8 {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if msg == "" {
		t.Fatal("expected guidance message")
	}
}

func TestExecImportWalletDatTextDump(t *testing.T) {
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
		ChainDataDir: dir2,
		WalletAddress: func() string { return w2.Address() },
		WalletImportSpendKey: func(w string) error {
			importedWIF = w
			return w2.ImportSpendPrivKey(w, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
		},
		WalletImportPrivKey: func(w string) error {
			return w2.ImportPrivKey(w, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
		},
		WalletImportWatch: func(script []byte) error { return w2.AddWatchScript(script) },
	}
	res, code, msg := execImportWalletDat("test", paths2, nil, nil, mustWalletJSONParam(t, dumpPath))
	if code != 0 {
		t.Fatalf("import: %s", msg)
	}
	if importedWIF != wif {
		t.Fatalf("wif mismatch %q vs %q", importedWIF, wif)
	}
	m, ok := res.(map[string]interface{})
	if !ok || m["imported_from"] != dumpPath {
		t.Fatalf("result %#v", res)
	}
}

func TestExecImportWalletDatViaCoreOptionJSON(t *testing.T) {
	dir := t.TempDir()
	bdb := filepath.Join(dir, "wallet.dat")
	page := make([]byte, 512)
	binary.LittleEndian.PutUint32(page[12:16], 0x00053162)
	binary.LittleEndian.PutUint32(page[20:24], 512)
	page[28] = 9
	if err := os.WriteFile(bdb, page, 0o600); err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		ChainDataDir:         dir,
		WalletImportSpendKey: func(string) error { return nil },
	}
	opts, _ := json.Marshal(map[string]bool{"via_core_rpc": true})
	pathParam, _ := json.Marshal(bdb)
	_, code, _ := execImportWalletDat("test", paths, nil, nil, []json.RawMessage{
		pathParam,
		opts,
	})
	if code != -1 {
		t.Fatalf("expected core_rpc not configured, code=%d", code)
	}
}

func TestExecImportWalletDatPassphraseOptionJSON(t *testing.T) {
	dir := t.TempDir()
	bdbPath := filepath.Join(dir, "wallet.dat")
	page := make([]byte, 512)
	binary.LittleEndian.PutUint32(page[12:16], 0x00053162)
	binary.LittleEndian.PutUint32(page[20:24], 512)
	page[28] = 9
	if err := os.WriteFile(bdbPath, page, 0o600); err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		ChainDataDir:         dir,
		WalletImportSpendKey: func(string) error { return nil },
	}
	opts, _ := json.Marshal(map[string]string{"passphrase": "s3cret"})
	pathParam, _ := json.Marshal(bdbPath)
	_, code, msg := execImportWalletDat("testnet", paths, nil, nil, []json.RawMessage{
		pathParam,
		opts,
	})
	if code != -8 {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if msg == "" || !strings.Contains(msg, "dogego_importwalletdat") {
		t.Fatalf("msg=%q", msg)
	}
}

func TestExecImportWalletDatNativeSyntheticFixture(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pub := append([]byte{0x03}, make([]byte, 32)...)
	secret := bytes.Repeat([]byte{0xdd}, 32)
	wantWIF, err := chain.EncodeWIF(secret, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	bdbPath := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestWalletDat(bdbPath, pub, secret); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	var importedWIF string
	paths := &DataPaths{
		ChainDataDir: dir,
		WalletAddress: func() string { return w.Address() },
		WalletImportSpendKey: func(wif string) error {
			importedWIF = wif
			return w.ImportSpendPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
		},
		WalletImportPrivKey: func(wif string) error {
			return w.ImportPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
		},
		WalletImportWatch: func(script []byte) error { return w.AddWatchScript(script) },
	}
	res, code, msg := execImportWalletDat("testnet", paths, nil, nil, mustWalletJSONParam(t, bdbPath))
	if code != 0 {
		t.Fatalf("import: code=%d msg=%q", code, msg)
	}
	if importedWIF != wantWIF {
		t.Fatalf("wif %q want %q", importedWIF, wantWIF)
	}
	m, ok := res.(map[string]interface{})
	if !ok || m["via_native_bdb"] != true {
		t.Fatalf("result %#v", res)
	}
}

func TestExecImportWalletDatEncryptedSyntheticFixture(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pub := append([]byte{0x02}, make([]byte, 32)...)
	secret := bytes.Repeat([]byte{0x11}, 32)
	passphrase := "fixture-pass"
	wantWIF, err := chain.EncodeWIF(secret, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	bdbPath := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestEncryptedWalletDat(bdbPath, pub, secret, passphrase); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	var importedWIF string
	paths := &DataPaths{
		ChainDataDir: dir,
		WalletAddress: func() string { return w.Address() },
		WalletImportSpendKey: func(wif string) error {
			importedWIF = wif
			return w.ImportSpendPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
		},
		WalletImportPrivKey: func(wif string) error {
			return w.ImportPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
		},
		WalletImportWatch: func(script []byte) error { return w.AddWatchScript(script) },
	}
	opts, _ := json.Marshal(map[string]string{"passphrase": passphrase})
	pathParam, _ := json.Marshal(bdbPath)
	res, code, msg := execImportWalletDat("testnet", paths, nil, nil, []json.RawMessage{pathParam, opts})
	if code != 0 {
		t.Fatalf("import: code=%d msg=%q", code, msg)
	}
	if importedWIF != wantWIF {
		t.Fatalf("wif %q want %q", importedWIF, wantWIF)
	}
	m, ok := res.(map[string]interface{})
	if !ok || m["via_native_bdb"] != true {
		t.Fatalf("result %#v", res)
	}
}

func TestExecImportWalletDatEncryptedDescriptorSyntheticFixture(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pub := append([]byte{0x02}, make([]byte, 32)...)
	secret := bytes.Repeat([]byte{0x22}, 32)
	passphrase := "descriptor-fixture-pass"
	wantWIF, err := chain.EncodeWIF(secret, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	bdbPath := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestEncryptedDescriptorWalletDat(bdbPath, pub, secret, passphrase); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	var importedWIF string
	paths := &DataPaths{
		ChainDataDir: dir,
		WalletAddress: func() string { return w.Address() },
		WalletImportSpendKey: func(wif string) error {
			importedWIF = wif
			return w.ImportSpendPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
		},
		WalletImportPrivKey: func(wif string) error {
			return w.ImportPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
		},
		WalletImportWatch: func(script []byte) error { return w.AddWatchScript(script) },
	}
	opts, _ := json.Marshal(map[string]string{"passphrase": passphrase})
	pathParam, _ := json.Marshal(bdbPath)
	res, code, msg := execImportWalletDat("testnet", paths, nil, nil, []json.RawMessage{pathParam, opts})
	if code != 0 {
		t.Fatalf("import: code=%d msg=%q", code, msg)
	}
	if importedWIF != wantWIF {
		t.Fatalf("wif %q want %q", importedWIF, wantWIF)
	}
	m, ok := res.(map[string]interface{})
	if !ok || m["via_native_bdb"] != true {
		t.Fatalf("result %#v", res)
	}
}

func TestExecImportWalletDatNativePoolMetadata(t *testing.T) {
	pub := append([]byte{0x03}, make([]byte, 32)...)
	secret := bytes.Repeat([]byte{0x44}, 32)
	bdbPath := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestWalletDatWithPool(bdbPath, pub, secret, 7); err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		ChainDataDir:         dir,
		WalletAddress:        func() string { return w.Address() },
		WalletImportSpendKey: func(wif string) error { return w.ImportSpendPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID) },
		WalletImportPrivKey:  func(wif string) error { return w.ImportPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID) },
		WalletImportWatch:    func(script []byte) error { return w.AddWatchScript(script) },
	}
	opts, _ := json.Marshal(map[string]bool{"native_bdb": true})
	pathParam, _ := json.Marshal(bdbPath)
	res, code, msg := execImportWalletDat("testnet", paths, nil, nil, []json.RawMessage{pathParam, opts})
	if code != 0 {
		t.Fatalf("import: code=%d msg=%q", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("type %T", res)
	}
	if fmt.Sprint(m["pool_count"]) != "1" {
		t.Fatalf("pool_count %#v", m["pool_count"])
	}
	if fmt.Sprint(m["pool_pubkeys"]) != "1" {
		t.Fatalf("pool_pubkeys %#v", m["pool_pubkeys"])
	}
	if fmt.Sprint(m["pool_keys_matched"]) != "1" {
		t.Fatalf("pool_keys_matched %#v", m["pool_keys_matched"])
	}
	if fmt.Sprint(m["pool_index_min"]) != "7" || fmt.Sprint(m["pool_index_max"]) != "7" {
		t.Fatalf("pool indices %#v", m)
	}
	if m["keypool_hint"] != corewallet.PoolKeypoolHint() {
		t.Fatalf("keypool_hint %#v", m["keypool_hint"])
	}
	if m["pool_indices_replayed"] != false {
		t.Fatalf("pool_indices_replayed %#v", m["pool_indices_replayed"])
	}
	entries, ok := m["pool_entries"].([]map[string]interface{})
	if !ok || len(entries) != 1 {
		t.Fatalf("pool_entries %#v", m["pool_entries"])
	}
}

func TestExecImportWalletDatNativePoolIndicesReplayed(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	const deepIndex uint32 = 150
	pub, priv, err := w.DeriveReceiveMaterial(deepIndex)
	if err != nil || priv == nil {
		t.Fatalf("derive: %v", err)
	}
	bdbPath := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestWalletDatWithPool(bdbPath, pub, priv.Serialize(), 7); err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		ChainDataDir:         dir,
		WalletAddress:        func() string { return w.Address() },
		WalletHDFormat:       func() string { return "hd" },
		WalletImportSpendKey: func(wif string) error { return w.ImportSpendPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID) },
		WalletImportPrivKey:  func(wif string) error { return w.ImportPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID) },
		WalletImportWatch:    func(script []byte) error { return w.AddWatchScript(script) },
		WalletReplayCorePool: func(entries []corewallet.PoolEntry) (wallet.PoolReplayResult, error) {
			return w.ReplayCorePoolIntoHDKeypool(entries)
		},
	}
	opts, _ := json.Marshal(map[string]bool{"native_bdb": true})
	pathParam, _ := json.Marshal(bdbPath)
	res, code, msg := execImportWalletDat("testnet", paths, nil, nil, []json.RawMessage{pathParam, opts})
	if code != 0 {
		t.Fatalf("import: code=%d msg=%q", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("type %T", res)
	}
	if m["pool_indices_replayed"] != true {
		t.Fatalf("pool_indices_replayed %#v", m["pool_indices_replayed"])
	}
	if !w.HDEnabled() {
		t.Fatal("HD wallet cleared during native wallet.dat import")
	}
	if w.KeypoolSize() < 100 {
		t.Fatalf("keypool=%d want >=100 after replay", w.KeypoolSize())
	}
}

func TestExecImportWalletDatMixedPoolMetadata(t *testing.T) {
	spendPub := append([]byte{0x03}, bytes.Repeat([]byte{0x55}, 32)...)
	poolOnlyPub := append([]byte{0x02}, bytes.Repeat([]byte{0x66}, 32)...)
	secret := bytes.Repeat([]byte{0x77}, 32)
	bdbPath := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestWalletDatWithMixedPool(bdbPath, spendPub, secret, poolOnlyPub, 3, 9); err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		ChainDataDir:         dir,
		WalletAddress:        func() string { return w.Address() },
		WalletImportSpendKey: func(wif string) error { return w.ImportSpendPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID) },
		WalletImportPrivKey:  func(wif string) error { return w.ImportPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID) },
		WalletImportWatch:    func(script []byte) error { return w.AddWatchScript(script) },
	}
	opts, _ := json.Marshal(map[string]bool{"native_bdb": true})
	pathParam, _ := json.Marshal(bdbPath)
	res, code, msg := execImportWalletDat("testnet", paths, nil, nil, []json.RawMessage{pathParam, opts})
	if code != 0 {
		t.Fatalf("import: code=%d msg=%q", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("type %T", res)
	}
	if fmt.Sprint(m["pool_keys_matched"]) != "1" || fmt.Sprint(m["pool_keys_unmatched"]) != "1" {
		t.Fatalf("pool match counts %#v", m)
	}
	if m["pool_unmatched_hint"] == "" {
		t.Fatalf("pool_unmatched_hint %#v", m["pool_unmatched_hint"])
	}
	if m["keypool_refill_size"] != float64(100) && m["keypool_refill_size"] != int(100) {
		t.Fatalf("keypool_refill_size %#v", m["keypool_refill_size"])
	}
	if m["pool_indices_replayed"] != false {
		t.Fatalf("pool_indices_replayed %#v", m["pool_indices_replayed"])
	}
}

func TestExecImportWalletDatMixedPoolHDReplay(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	spendPub, ok := w.CompressedPubKeyForAddress(w.Address())
	if !ok {
		t.Fatal("missing HD receive pubkey")
	}
	priv, err := w.PrivKeyForAddress(w.Address())
	if err != nil || priv == nil {
		t.Fatalf("priv: %v", err)
	}
	poolOnlyPub := append([]byte{0x02}, bytes.Repeat([]byte{0xab}, 32)...)
	bdbPath := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestWalletDatWithMixedPool(bdbPath, spendPub, priv.Serialize(), poolOnlyPub, 3, 9); err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		ChainDataDir:         dir,
		WalletAddress:        func() string { return w.Address() },
		WalletHDFormat:       func() string { return "hd" },
		WalletImportSpendKey: func(wif string) error { return w.ImportSpendPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID) },
		WalletImportPrivKey:  func(wif string) error { return w.ImportPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID) },
		WalletImportWatch:    func(script []byte) error { return w.AddWatchScript(script) },
		WalletReplayCorePool: func(entries []corewallet.PoolEntry) (wallet.PoolReplayResult, error) {
			return w.ReplayCorePoolIntoHDKeypool(entries)
		},
	}
	opts, _ := json.Marshal(map[string]bool{"native_bdb": true})
	pathParam, _ := json.Marshal(bdbPath)
	res, code, msg := execImportWalletDat("testnet", paths, nil, nil, []json.RawMessage{pathParam, opts})
	if code != 0 {
		t.Fatalf("import: code=%d msg=%q", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("type %T", res)
	}
	if fmt.Sprint(m["pool_keys_matched"]) != "1" || fmt.Sprint(m["pool_keys_unmatched"]) != "1" {
		t.Fatalf("pool match counts %#v", m)
	}
	// Default receive (index 0) is already issued: core index is stored, but not re-queued.
	if m["pool_indices_replayed"] != false && m["pool_indices_replayed"] != nil {
		t.Fatalf("pool_indices_replayed %#v", m["pool_indices_replayed"])
	}
	if m["pool_core_indices_stored"] != float64(1) && m["pool_core_indices_stored"] != int(1) {
		t.Fatalf("pool_core_indices_stored %#v", m["pool_core_indices_stored"])
	}
	if !w.HDEnabled() {
		t.Fatal("HD wallet cleared during mixed-pool import")
	}
}

func TestExecImportWalletDatCallsKeypoolRefill(t *testing.T) {
	pub := append([]byte{0x03}, make([]byte, 32)...)
	secret := bytes.Repeat([]byte{0x44}, 32)
	bdbPath := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestWalletDatWithPool(bdbPath, pub, secret, 7); err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	var refilled bool
	paths := &DataPaths{
		ChainDataDir:         dir,
		WalletAddress:        func() string { return w.Address() },
		WalletImportSpendKey: func(wif string) error { return w.ImportSpendPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID) },
		WalletImportPrivKey:  func(wif string) error { return w.ImportPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID) },
		WalletImportWatch:    func(script []byte) error { return w.AddWatchScript(script) },
		WalletWatchScripts:   func() [][]byte { return w.WatchScripts() },
		WalletKeypoolRefill: func(n int) error {
			refilled = true
			return nil
		},
	}
	opts, _ := json.Marshal(map[string]bool{"native_bdb": true})
	pathParam, _ := json.Marshal(bdbPath)
	_, code, msg := execImportWalletDat("testnet", paths, nil, nil, []json.RawMessage{pathParam, opts})
	if code != 0 {
		t.Fatalf("import: code=%d msg=%q", code, msg)
	}
	if !refilled {
		t.Fatal("expected keypool refill after native wallet.dat import")
	}
}

func TestExecImportWalletDatMultiKeyNativeFixture(t *testing.T) {
	pub1 := append([]byte{0x02}, bytes.Repeat([]byte{0x01}, 32)...)
	secret1 := bytes.Repeat([]byte{0x11}, 32)
	pub2 := append([]byte{0x03}, bytes.Repeat([]byte{0x02}, 32)...)
	secret2 := bytes.Repeat([]byte{0x22}, 32)
	bdbPath := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestWalletDatMultiKey(bdbPath, pub1, secret1, pub2, secret2); err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	wif1, err := chain.EncodeWIF(secret1, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	wif2, err := chain.EncodeWIF(secret2, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		ChainDataDir:         dir,
		WalletAddress:        func() string { return w.Address() },
		WalletImportSpendKey: func(wif string) error { return w.ImportSpendPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID) },
		WalletImportPrivKey:  func(wif string) error { return w.ImportPrivKey(wif, p.PrivKeyWIFVersion, p.PubkeyHashAddrID) },
		WalletImportWatch:    func(script []byte) error { return w.AddWatchScript(script) },
	}
	opts, _ := json.Marshal(map[string]bool{"native_bdb": true})
	pathParam, _ := json.Marshal(bdbPath)
	res, code, msg := execImportWalletDat("testnet", paths, nil, nil, []json.RawMessage{pathParam, opts})
	if code != 0 {
		t.Fatalf("import: code=%d msg=%q", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok || fmt.Sprint(m["keys_imported"]) != "2" {
		t.Fatalf("result %#v", res)
	}
	all, err := w.AllWIFs(p.PrivKeyWIFVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("wifs=%v", all)
	}
	seen := map[string]bool{all[0]: true, all[1]: true}
	if !seen[wif1] || !seen[wif2] {
		t.Fatalf("missing keys in %v", all)
	}
}
