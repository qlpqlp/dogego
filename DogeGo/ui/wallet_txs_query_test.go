// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dogego/wallet"
)

func TestParseWalletTxListQueryDefaults(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/wallet/txs", nil)
	offset, limit, q, kind := parseWalletTxListQuery(req)
	if offset != 0 || limit != walletTxDefaultLimit || q != "" || kind != "all" {
		t.Fatalf("defaults offset=%d limit=%d q=%q kind=%q", offset, limit, q, kind)
	}
}

func TestParseWalletTxListQueryPaging(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/wallet/txs?offset=80&limit=25&q=abc&type=sent", nil)
	offset, limit, q, kind := parseWalletTxListQuery(req)
	if offset != 80 || limit != 25 || q != "abc" || kind != "sent" {
		t.Fatalf("got offset=%d limit=%d q=%q kind=%q", offset, limit, q, kind)
	}
}

func TestParseWalletTxListQueryLimitCap(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/wallet/txs?limit=9999", nil)
	_, limit, _, _ := parseWalletTxListQuery(req)
	if limit != walletTxMaxLimit {
		t.Fatalf("limit %d want %d", limit, walletTxMaxLimit)
	}
}

func TestWalletTxsBridgeListPage(t *testing.T) {
	b := &WalletTxsBridge{}
	b.SetPage(func(offset, limit int, q, kind string) WalletTxPageResult {
		if offset != 40 || limit != 10 || q != "mine" || kind != "mining" {
			t.Fatalf("pageFn args offset=%d limit=%d q=%q kind=%q", offset, limit, q, kind)
		}
		return WalletTxPageResult{Total: 100, Offset: 40, Limit: 10, Items: []interface{}{map[string]interface{}{"txid": "x"}}}
	})
	page, ok := b.ListPage(40, 10, "mine", "mining")
	if !ok || page.Total != 100 || len(page.Items) != 1 {
		t.Fatalf("page %#v ok=%v", page, ok)
	}
}

func TestWalletTxsHTTPPagination(t *testing.T) {
	mux := http.NewServeMux()
	var stub wallet.Disk
	bridge := &WalletTxsBridge{}
	bridge.SetPage(func(offset, limit int, q, kind string) WalletTxPageResult {
		return WalletTxPageResult{
			Total:  200,
			Offset: offset,
			Limit:  limit,
			Items:  []interface{}{map[string]interface{}{"txid": "abc", "offset": offset, "q": q, "kind": kind}},
		}
	})
	registerWalletTxsRoutes(mux, StartConfig{Wallet: &stub, WalletTxs: bridge}, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/wallet/txs?offset=40&limit=20&q=foo&type=sent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out["total"].(float64)) != 200 || int(out["offset"].(float64)) != 40 {
		t.Fatalf("page meta %#v", out)
	}
	items, ok := out["items"].([]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("items %#v", out["items"])
	}
}

func TestWalletTxsHTTPFirstPageLargeLimit(t *testing.T) {
	mux := http.NewServeMux()
	var stub wallet.Disk
	bridge := &WalletTxsBridge{}
	bridge.SetPage(func(offset, limit int, q, kind string) WalletTxPageResult {
		if limit != walletTxMaxLimit {
			t.Fatalf("limit=%d want cap %d", limit, walletTxMaxLimit)
		}
		items := make([]interface{}, 0, limit)
		for i := 0; i < limit; i++ {
			items = append(items, map[string]interface{}{"txid": "t", "confirmations": float64(i)})
		}
		return WalletTxPageResult{Total: 500, Offset: offset, Limit: limit, Items: items}
	})
	registerWalletTxsRoutes(mux, StartConfig{Wallet: &stub, WalletTxs: bridge}, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/wallet/txs?limit=500&offset=0")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	items, ok := out["items"].([]interface{})
	if !ok || len(items) != walletTxMaxLimit {
		t.Fatalf("items len=%d want %d", len(items), walletTxMaxLimit)
	}
}

func TestWalletTxsHTTPPartialPageMeta(t *testing.T) {
	mux := http.NewServeMux()
	var stub wallet.Disk
	bridge := &WalletTxsBridge{}
	bridge.SetPage(func(offset, limit int, q, kind string) WalletTxPageResult {
		items := make([]interface{}, 0, limit)
		for i := 0; i < limit; i++ {
			items = append(items, map[string]interface{}{"txid": "tx", "offset": offset + i})
		}
		return WalletTxPageResult{Total: 500, Offset: offset, Limit: limit, Items: items}
	})
	registerWalletTxsRoutes(mux, StartConfig{Wallet: &stub, WalletTxs: bridge}, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/wallet/txs?offset=200&limit=40")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out["total"].(float64)) != 500 || int(out["offset"].(float64)) != 200 || int(out["limit"].(float64)) != 40 {
		t.Fatalf("meta %#v", out)
	}
	items, ok := out["items"].([]interface{})
	if !ok || len(items) != 40 {
		t.Fatalf("items len=%d", len(items))
	}
	remaining := int(out["total"].(float64)) - int(out["offset"].(float64)) - len(items)
	if remaining != 260 {
		t.Fatalf("remaining %d want 260 for Load-all UI", remaining)
	}
}

func TestWalletTxsCSVEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	var stub wallet.Disk
	bridge := &WalletTxsBridge{}
	bridge.SetPage(func(offset, limit int, q, kind string) WalletTxPageResult {
		return WalletTxPageResult{
			Total: 1,
			Items: []interface{}{map[string]interface{}{"txid": "csvtx", "amount": 1.0, "time": float64(1700000000)}},
		}
	})
	registerWalletTxsRoutes(mux, StartConfig{Wallet: &stub, WalletTxs: bridge}, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/wallet/txs.csv?q=&kind=all")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Fatalf("content-type %q", ct)
	}
}
