// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dogego/wallet"
)

func testWalletImportServer(t *testing.T, invoke func(string, []json.RawMessage) map[string]interface{}) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var stub wallet.Disk
	registerWalletImportRoutes(mux, StartConfig{
		Wallet:    &stub,
		RPCInvoke: invoke,
	}, nil)
	return httptest.NewServer(mux)
}

func TestWalletImportMnemonicAPI(t *testing.T) {
	var gotMethod string
	srv := testWalletImportServer(t, func(method string, params []json.RawMessage) map[string]interface{} {
		gotMethod = method
		if method != "dogego_importmnemonic" {
			t.Fatalf("method %s", method)
		}
		return map[string]interface{}{
			"result": map[string]any{"ok": true, "address": "DImportAddr", "hd": true},
		}
	})
	defer srv.Close()

	body := `{"type":"mnemonic","mnemonic":"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about","passphrase":"","rescan":false}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/wallet/import", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["ok"] != true {
		t.Fatalf("response %#v", out)
	}
	if gotMethod != "dogego_importmnemonic" {
		t.Fatalf("rpc method %q", gotMethod)
	}
}

func TestWalletAddressNewAPI(t *testing.T) {
	var gotMethod string
	srv := testWalletImportServer(t, func(method string, params []json.RawMessage) map[string]interface{} {
		gotMethod = method
		if method != "getnewaddress" {
			t.Fatalf("method %s", method)
		}
		return map[string]interface{}{"result": "DNewAddr"}
	})
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/wallet/address/new", strings.NewReader(`{"label":"shop"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["ok"] != true || out["result"] != "DNewAddr" {
		t.Fatalf("response %#v", out)
	}
	if gotMethod != "getnewaddress" {
		t.Fatalf("rpc method %q", gotMethod)
	}
}

func TestWalletAddressNewAPIWarmup(t *testing.T) {
	srv := testWalletImportServer(t, func(method string, params []json.RawMessage) map[string]interface{} {
		return map[string]interface{}{
			"error": map[string]interface{}{
				"code":    -1,
				"message": "getnewaddress: wallet is not implemented in DogeGo",
			},
		}
	})
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/wallet/address/new", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d want 503", resp.StatusCode)
	}
}

func TestWalletAddressLabelAPI(t *testing.T) {
	srv := testWalletImportServer(t, func(method string, params []json.RawMessage) map[string]interface{} {
		if method != "setlabel" {
			t.Fatalf("method %s", method)
		}
		return map[string]interface{}{"result": nil}
	})
	defer srv.Close()

	body := `{"address":"DRecv","label":"tips"}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/wallet/address/label", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
}

func TestWalletImportBIP38API(t *testing.T) {
	srv := testWalletImportServer(t, func(method string, params []json.RawMessage) map[string]interface{} {
		if method != "dogego_importbip38" {
			t.Fatalf("method %s", method)
		}
		return map[string]interface{}{
			"result": map[string]any{"ok": true, "address": "DSwept"},
		}
	})
	defer srv.Close()

	body := `{"type":"bip38","bip38":"6PTest","passphrase":"secret","rescan":false}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/wallet/import", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
}

func TestWalletLabelsAPI(t *testing.T) {
	srv := testWalletImportServer(t, func(method string, params []json.RawMessage) map[string]interface{} {
		if method != "listlabels" {
			t.Fatalf("method %s", method)
		}
		return map[string]interface{}{
			"result": []string{"shop", "tips"},
		}
	})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/wallet/labels")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var labels []string
	if err := json.NewDecoder(resp.Body).Decode(&labels); err != nil {
		t.Fatal(err)
	}
	if len(labels) != 2 || labels[0] != "shop" {
		t.Fatalf("labels %#v", labels)
	}
}

func TestWalletImportAddressesAPI(t *testing.T) {
	srv := testWalletImportServer(t, func(method string, params []json.RawMessage) map[string]interface{} {
		if method != "dogego_listwalletaddresses" {
			t.Fatalf("method %s", method)
		}
		return map[string]interface{}{
			"result": []map[string]any{
				{"address": "DReceive", "hd_path": "m/44'/3'/0'/0/0"},
			},
		}
	})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/wallet/addresses")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var rows []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["address"] != "DReceive" {
		t.Fatalf("rows %#v", rows)
	}
}

func TestWalletImportWalletDatAPI(t *testing.T) {
	srv := testWalletImportServer(t, func(method string, params []json.RawMessage) map[string]interface{} {
		if method != "dogego_importwalletdat" {
			t.Fatalf("method %s", method)
		}
		return map[string]interface{}{
			"result": map[string]any{"via_native_bdb": true, "keys_imported": float64(2)},
		}
	})
	defer srv.Close()

	body := `{"type":"walletdat","path":"C:\\wallet.dat","via_core_rpc":false}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/wallet/import", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
}

func TestWalletImportWalletDatPassphraseAPI(t *testing.T) {
	srv := testWalletImportServer(t, func(method string, params []json.RawMessage) map[string]interface{} {
		if method != "dogego_importwalletdat" {
			t.Fatalf("method %s", method)
		}
		if len(params) != 2 {
			t.Fatalf("params len %d", len(params))
		}
		var opts map[string]interface{}
		if err := json.Unmarshal(params[1], &opts); err != nil {
			t.Fatal(err)
		}
		if opts["passphrase"] != "s3cret" {
			t.Fatalf("passphrase %#v", opts["passphrase"])
		}
		return map[string]interface{}{
			"result": map[string]any{"via_native_bdb": true, "keys_imported": float64(1)},
		}
	})
	defer srv.Close()

	body := `{"type":"walletdat","path":"C:\\wallet.dat","passphrase":"s3cret"}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/wallet/import", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
}

func TestWalletProbeWalletDatAPI(t *testing.T) {
	srv := testWalletImportServer(t, func(method string, params []json.RawMessage) map[string]interface{} {
		if method != "dogego_probewalletdat" {
			t.Fatalf("method %s", method)
		}
		return map[string]interface{}{
			"result": map[string]any{
				"is_bdb":          true,
				"key_count":       float64(1),
				"can_import":      true,
				"pool_count":      float64(2),
				"pool_pubkeys":    float64(2),
				"pool_index_min":  float64(4),
				"pool_index_max":  float64(8),
			},
		}
	})
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/wallet/probe-walletdat?path=C%3A%5Cwallet.dat", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	res, _ := body["result"].(map[string]any)
	if res == nil || res["pool_count"] != float64(2) || res["pool_pubkeys"] != float64(2) {
		t.Fatalf("result %#v", body)
	}
}

func TestWalletImportForbiddenNonLoopback(t *testing.T) {
	mux := http.NewServeMux()
	var stub wallet.Disk
	registerWalletImportRoutes(mux, StartConfig{
		Wallet:    &stub,
		RPCInvoke: func(string, []json.RawMessage) map[string]interface{} { return nil },
	}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/wallet/import", strings.NewReader(`{"type":"mnemonic","mnemonic":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.1:1234"
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status %d want 403", rr.Code)
	}
}

func TestWalletImportDisabledWithoutWallet(t *testing.T) {
	mux := http.NewServeMux()
	registerWalletImportRoutes(mux, StartConfig{
		RPCInvoke: func(string, []json.RawMessage) map[string]interface{} { return nil },
	}, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/wallet/import", strings.NewReader(`{"type":"mnemonic","mnemonic":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d want 400", resp.StatusCode)
	}
}
