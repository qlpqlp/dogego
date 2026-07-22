// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestWalletBalanceFromMap(t *testing.T) {
	if walletBalance(map[string]any{"balance": 12.5}) != 12.5 {
		t.Fatal("float balance")
	}
	if walletBalance(nil) != 0 {
		t.Fatal("nil balance")
	}
}

func TestInvokeJSONRPCStringAndSlice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		switch req.Method {
		case "getnewaddress":
			_, _ = w.Write([]byte(`{"result":"DAddrBootstrap"}`))
		case "generatetoaddress":
			_, _ = w.Write([]byte(`{"result":["abc123"]}`))
		default:
			t.Fatalf("method %q", req.Method)
		}
	}))
	defer srv.Close()
	_, portStr, err := netSplitHostPortFromURL(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := invokeJSONRPCString("127.0.0.1", port, "", "", "getnewaddress", []any{"label"}, 0)
	if err != nil || addr != "DAddrBootstrap" {
		t.Fatalf("addr=%q err=%v", addr, err)
	}
	hashes, err := invokeJSONRPCStringSlice("127.0.0.1", port, "", "", "generatetoaddress", []any{2, addr}, 0)
	if err != nil || len(hashes) != 1 || hashes[0] != "abc123" {
		t.Fatalf("hashes=%v err=%v", hashes, err)
	}
}

func TestVerifySetupParityPreflightFailure(t *testing.T) {
	res := VerifySetupParity(SetupParityOptions{DogeGoPort: 1, CorePort: 2})
	if res.OK || len(res.Issues) == 0 {
		t.Fatalf("expected failure: %+v", res)
	}
	if res.Doc != DogegoLiveWorkflow10Doc {
		t.Fatalf("doc %q", res.Doc)
	}
}
