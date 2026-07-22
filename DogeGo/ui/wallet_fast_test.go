// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/store"
	"dogego/wallet"
)

func testWalletFastSetup(t *testing.T) (StartConfig, string, []byte) {
	t.Helper()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	addr := w.Address()
	_, payload, err := chain.Base58CheckDecode(addr)
	if err != nil {
		t.Fatal(err)
	}
	spk := chain.P2PKHScriptFromPubKeyHash(payload)
	u := store.NewUtxoCache()
	u.SetTipHeightForTest(300)
	cfg := StartConfig{
		Network: "testnet",
		Wallet:  w,
		UtxoCache: func() *store.UtxoCache {
			return u
		},
	}
	return cfg, addr, spk
}

func addWalletFastUtxo(u *store.UtxoCache, txByte byte, vout uint32, value int64, height int64, spk []byte) {
	var op [36]byte
	var h [32]byte
	h[0] = txByte
	copy(op[:32], h[:])
	binary.LittleEndian.PutUint32(op[32:], vout)
	u.AddUtxoForTest(op, store.UtxoEntry{Value: value, PkScript: spk, Height: height})
}

func TestWalletBalanceFromUtxoCacheConfirmedImmature(t *testing.T) {
	cfg, _, spk := testWalletFastSetup(t)
	u := cfg.UtxoCache()
	// Immature coinbase (vout 0, conf 201 < 240 maturity at tip 300).
	addWalletFastUtxo(u, 1, 0, 10_000_000_000, 100, spk)
	// Mature coinbase.
	addWalletFastUtxo(u, 2, 0, 5_000_000_000, 50, spk)
	// Regular receive (vout 1 counts as confirmed even when young).
	addWalletFastUtxo(u, 3, 1, 1_000_000_000, 290, spk)

	confirmed, immature, count, ok := walletBalanceFromUtxoCache(cfg)
	if !ok {
		t.Fatal("expected ok")
	}
	if count != 3 {
		t.Fatalf("utxo count=%d want 3", count)
	}
	if immature != 100.0 {
		t.Fatalf("immature=%v want 100", immature)
	}
	if confirmed != 60.0 {
		t.Fatalf("confirmed=%v want 60 (50+10)", confirmed)
	}
}

func TestWalletBalanceFromUtxoCacheNoMatch(t *testing.T) {
	cfg, _, spk := testWalletFastSetup(t)
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	other, _ := chain.RandomP2PKHAddress(p)
	_, payload, _ := chain.Base58CheckDecode(other)
	otherSpk := chain.P2PKHScriptFromPubKeyHash(payload)
	addWalletFastUtxo(cfg.UtxoCache(), 1, 0, 1e8, 10, otherSpk)
	confirmed, immature, count, ok := walletBalanceFromUtxoCache(cfg)
	if !ok {
		t.Fatal("expected ok when cache is live")
	}
	if count != 0 || confirmed != 0 || immature != 0 {
		t.Fatalf("count=%d confirmed=%v immature=%v", count, confirmed, immature)
	}
	_ = spk
}

func TestWalletAPIEnvelopeZeroUtxosWhenCacheLive(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	out := walletAPIEnvelope(cfg)
	if out["balance"].(float64) != 0 {
		t.Fatalf("balance %#v", out["balance"])
	}
	if n, ok := out["utxo_count"].(int); !ok || n != 0 {
		if f, ok := out["utxo_count"].(float64); !ok || int(f) != 0 {
			t.Fatalf("utxo_count %#v", out["utxo_count"])
		}
	}
}

func TestWalletTxPageFromUtxoCachePaginationAndFilters(t *testing.T) {
	cfg, _, spk := testWalletFastSetup(t)
	u := cfg.UtxoCache()
	addWalletFastUtxo(u, 1, 0, 10_000_000_000, 100, spk) // mining_immature
	addWalletFastUtxo(u, 2, 0, 5_000_000_000, 50, spk)   // mining (mature coinbase)
	addWalletFastUtxo(u, 3, 1, 2_000_000_000, 200, spk)  // received

	total, items, ok := walletTxPageFromUtxoCache(cfg, 0, 2, "", "all")
	if !ok || total != 3 || len(items) != 2 {
		t.Fatalf("page all total=%d items=%d ok=%v", total, len(items), ok)
	}

	total, items, ok = walletTxPageFromUtxoCache(cfg, 0, 0, "", "mining")
	if !ok || total != 2 || len(items) != 2 {
		t.Fatalf("mining filter total=%d items=%d", total, len(items))
	}
	row, _ := items[0].(map[string]interface{})
	if row["tx_kind"] != "mining_immature" {
		t.Fatalf("kind %#v", row["tx_kind"])
	}

	total, items, ok = walletTxPageFromUtxoCache(cfg, 0, 10, "", "received")
	if !ok || total != 1 || len(items) != 1 {
		t.Fatalf("received page total=%d items=%d", total, len(items))
	}

	total, _, ok = walletTxPageFromUtxoCache(cfg, 0, 0, "50", "all")
	if !ok || total != 1 {
		t.Fatalf("search by amount total=%d", total)
	}
}

func TestWalletAPIEnvelopeUsesUtxoCache(t *testing.T) {
	cfg, _, spk := testWalletFastSetup(t)
	// Mature receive (vout 1) so balance lands in confirmed, not immature.
	addWalletFastUtxo(cfg.UtxoCache(), 1, 1, 3_000_000_000, 80, spk)
	out := walletAPIEnvelope(cfg)
	if out["balance"].(float64) != 30.0 {
		t.Fatalf("balance %#v", out["balance"])
	}
	if n, ok := out["utxo_count"].(int); !ok || n != 1 {
		if f, ok := out["utxo_count"].(float64); !ok || int(f) != 1 {
			t.Fatalf("utxo_count %#v", out["utxo_count"])
		}
	}
}

func TestWalletTxsHTTPUsesUtxoCacheFastPath(t *testing.T) {
	cfg, _, spk := testWalletFastSetup(t)
	addWalletFastUtxo(cfg.UtxoCache(), 9, 0, 1_000_000_000, 150, spk)
	mux := http.NewServeMux()
	registerWalletTxsRoutes(mux, cfg, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/wallet/txs?limit=10&offset=0&type=mining")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out["total"].(float64)) != 1 {
		t.Fatalf("total %#v", out["total"])
	}
	items, ok := out["items"].([]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("items %#v", out["items"])
	}
}

func TestWalletTxsCSVUsesUtxoCacheFastPath(t *testing.T) {
	cfg, _, spk := testWalletFastSetup(t)
	addWalletFastUtxo(cfg.UtxoCache(), 4, 1, 2_500_000_000, 200, spk)
	mux := http.NewServeMux()
	registerWalletTxsRoutes(mux, cfg, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/wallet/txs.csv?kind=received")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	body := make([]byte, 4096)
	n, _ := resp.Body.Read(body)
	csv := string(body[:n])
	if !strings.Contains(csv, "amount_doge") || !strings.Contains(csv, "25") {
		t.Fatalf("csv %q", csv)
	}
}

func TestWalletHTTPUsesUtxoCacheFastPath(t *testing.T) {
	cfg, _, spk := testWalletFastSetup(t)
	addWalletFastUtxo(cfg.UtxoCache(), 7, 1, 6_000_000_000, 220, spk)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/wallet", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(walletAPIEnvelope(cfg))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/wallet")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["balance"].(float64) != 60.0 {
		t.Fatalf("balance %#v", out["balance"])
	}
}

func TestWalletUtxosHTTPUsesUtxoCacheFastPath(t *testing.T) {
	cfg, _, spk := testWalletFastSetup(t)
	addWalletFastUtxo(cfg.UtxoCache(), 5, 1, 4_000_000_000, 180, spk)
	addWalletFastUtxo(cfg.UtxoCache(), 6, 0, 10_000_000_000, 150, spk) // immature coinbase
	mux := http.NewServeMux()
	registerWalletUtxosRoute(mux, cfg, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/wallet/utxos")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows=%d want 2", len(rows))
	}
	var spendable, immature int
	for _, r := range rows {
		if r["spendable"] == true {
			spendable++
		} else {
			immature++
		}
	}
	if spendable != 1 || immature != 1 {
		t.Fatalf("spendable=%d immature=%d", spendable, immature)
	}
}

func TestWalletListUnspentFromUtxoCacheSpendableSplit(t *testing.T) {
	cfg, _, spk := testWalletFastSetup(t)
	addWalletFastUtxo(cfg.UtxoCache(), 1, 0, 1e9, 150, spk)
	addWalletFastUtxo(cfg.UtxoCache(), 2, 1, 2e9, 200, spk)
	rows, ok := walletListUnspentFromUtxoCache(cfg)
	if !ok || len(rows) != 2 {
		t.Fatalf("ok=%v len=%d", ok, len(rows))
	}
	var spendable int
	for _, row := range rows {
		m := row.(map[string]interface{})
		if m["spendable"] == true {
			spendable++
		}
	}
	if spendable != 1 {
		t.Fatalf("spendable=%d want 1 mature vout", spendable)
	}
}

func TestWalletUtxosHTTPEmptyWhenCacheLive(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	mux := http.NewServeMux()
	registerWalletUtxosRoute(mux, cfg, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/wallet/utxos")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var rows []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows=%d want 0", len(rows))
	}
}

func TestWalletListUnspentFromUtxoCacheEmptyReturnsOk(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	rows, ok := walletListUnspentFromUtxoCache(cfg)
	if !ok {
		t.Fatal("expected ok with live empty cache")
	}
	if len(rows) != 0 {
		t.Fatalf("rows=%d want 0", len(rows))
	}
}

func TestWalletListUnspentFromUtxoCacheAllSpendScripts(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.RestoreFromMnemonic("legal winner thank year wave sausage worth useful legal winner thank yellow", ""); err != nil {
		t.Fatal(err)
	}
	a1, err := w.NewReceiveAddress()
	if err != nil {
		t.Fatal(err)
	}
	if a1 == w.Address() {
		t.Fatal("expected distinct receive address")
	}
	scripts := w.SpendScripts()
	if len(scripts) < 2 {
		t.Fatalf("spend scripts %d want >= 2", len(scripts))
	}
	var spk1 []byte
	for _, s := range scripts {
		addr := chain.ScriptPubKeyAddress(s, p.PubkeyHashAddrID, p.ScriptHashAddrID)
		if addr == a1 {
			spk1 = append([]byte(nil), s...)
			break
		}
	}
	if len(spk1) == 0 {
		t.Fatal("missing script for new receive address")
	}
	u := store.NewUtxoCache()
	u.SetTipHeightForTest(300)
	cfg := StartConfig{
		Network: "testnet",
		Wallet:  w,
		UtxoCache: func() *store.UtxoCache {
			return u
		},
	}
	addWalletFastUtxo(u, 1, 0, 1_000_000_000, 150, spk1)
	rows, ok := walletListUnspentFromUtxoCache(cfg)
	if !ok || len(rows) != 1 {
		t.Fatalf("ok=%v len=%d", ok, len(rows))
	}
}

func TestWalletUtxosHTTPAllSpendScripts(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.RestoreFromMnemonic("legal winner thank year wave sausage worth useful legal winner thank yellow", ""); err != nil {
		t.Fatal(err)
	}
	a1, err := w.NewReceiveAddress()
	if err != nil {
		t.Fatal(err)
	}
	var spk1 []byte
	for _, s := range w.SpendScripts() {
		if chain.ScriptPubKeyAddress(s, p.PubkeyHashAddrID, p.ScriptHashAddrID) == a1 {
			spk1 = append([]byte(nil), s...)
			break
		}
	}
	if len(spk1) == 0 {
		t.Fatal("missing script for HD receive address")
	}
	u := store.NewUtxoCache()
	u.SetTipHeightForTest(300)
	cfg := StartConfig{
		Network: "testnet",
		Wallet:  w,
		UtxoCache: func() *store.UtxoCache {
			return u
		},
	}
	addWalletFastUtxo(u, 9, 0, 5_000_000_000, 200, spk1)
	mux := http.NewServeMux()
	registerWalletUtxosRoute(mux, cfg, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/wallet/utxos")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%d want 1 (HD spend script not default address)", len(rows))
	}
	if rows[0]["address"] != a1 {
		t.Fatalf("address=%q want %q", rows[0]["address"], a1)
	}
}

func TestFilterWalletUtxoRowsKindMatching(t *testing.T) {
	rows := []walletUtxoTxRow{
		{txid: "a", vout: 0, confirmations: 10},
		{txid: "b", vout: 1, confirmations: 10},
	}
	maturity := int64(240)
	out := filterWalletUtxoRows(rows, "", "mining", maturity)
	if len(out) != 1 || out[0].txid != "a" {
		t.Fatalf("mining filter %#v", out)
	}
	out = filterWalletUtxoRows(rows, "", "received", maturity)
	if len(out) != 1 || out[0].txid != "b" {
		t.Fatalf("received filter %#v", out)
	}
}

func TestWalletTxHistoryUsesUtxoFastPath(t *testing.T) {
	if !walletTxHistoryUsesUtxoFastPath("mining") {
		t.Fatal("mining should use fast path")
	}
	if !walletTxHistoryUsesUtxoFastPath("received") {
		t.Fatal("received should use fast path")
	}
	if !walletTxHistoryUsesUtxoFastPath("all") {
		t.Fatal("all should use fast path")
	}
	if walletTxHistoryUsesUtxoFastPath("quantum") {
		t.Fatal("quantum must not use fast path")
	}
	if walletTxHistoryUsesUtxoFastPath("sent") {
		t.Fatal("sent must not use fast path")
	}
}

func TestWalletTxsHTTPQuantumBypassesUtxoFastPath(t *testing.T) {
	cfg, _, spk := testWalletFastSetup(t)
	addWalletFastUtxo(cfg.UtxoCache(), 9, 0, 1_000_000_000, 150, spk)
	bridge := &WalletTxsBridge{}
	bridge.SetPage(func(offset, limit int, q, kind string) WalletTxPageResult {
		if kind != "quantum" {
			t.Fatalf("kind=%q want quantum", kind)
		}
		return WalletTxPageResult{
			Total: 1,
			Items: []interface{}{
				map[string]interface{}{"txid": "pqtx", "tx_kind": "sent_pq", "pq_tag": "FLC1"},
			},
		}
	})
	cfg.WalletTxs = bridge
	mux := http.NewServeMux()
	registerWalletTxsRoutes(mux, cfg, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/wallet/txs?limit=10&type=quantum")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out["total"].(float64)) != 1 {
		t.Fatalf("total %#v", out["total"])
	}
	items := out["items"].([]interface{})
	row := items[0].(map[string]interface{})
	if row["tx_kind"] != "sent_pq" {
		t.Fatalf("row %#v", row)
	}
}
