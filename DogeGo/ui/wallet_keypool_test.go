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

func testWalletKeypoolServer(t *testing.T, invoke func(string, []json.RawMessage) map[string]interface{}) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var stub wallet.Disk
	registerWalletKeypoolRoute(mux, StartConfig{
		Wallet:    &stub,
		RPCInvoke: invoke,
	}, nil)
	return httptest.NewServer(mux)
}

func TestWalletKeypoolRefillAPI(t *testing.T) {
	var gotMethod string
	var gotParams []json.RawMessage
	srv := testWalletKeypoolServer(t, func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "keypoolrefill":
			gotMethod = method
			gotParams = params
			return map[string]interface{}{"result": true}
		case "getwalletinfo":
			return map[string]interface{}{
				"result": map[string]any{
					"keypoolsize":              float64(100),
					"keypoolsize_hd_internal":  float64(100),
					"pool_core_indices_stored": float64(3),
				},
			}
		default:
			t.Fatalf("unexpected method %s", method)
			return nil
		}
	})
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/wallet/keypool-refill", strings.NewReader(`{"new_size":100}`))
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
	if gotMethod != "keypoolrefill" {
		t.Fatalf("rpc method %q", gotMethod)
	}
	if len(gotParams) != 1 || string(gotParams[0]) != "100" {
		t.Fatalf("params %#v", gotParams)
	}
	if out["keypool_size"] != float64(100) {
		t.Fatalf("keypool_size=%v", out["keypool_size"])
	}
}

func TestWalletKeypoolRefillForbiddenNonLoopback(t *testing.T) {
	mux := http.NewServeMux()
	var stub wallet.Disk
	registerWalletKeypoolRoute(mux, StartConfig{
		Wallet:    &stub,
		RPCInvoke: func(string, []json.RawMessage) map[string]interface{} {
			return map[string]interface{}{"result": true}
		},
	}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/wallet/keypool-refill", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.1:1234"
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status %d want 403", rr.Code)
	}
}
