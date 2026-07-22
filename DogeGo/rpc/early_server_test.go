// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dogego/httptls"
)

func TestEarlyServerWarmupThenActivate(t *testing.T) {
	es := &EarlyServer{}
	srv := httptest.NewServer(es)
	defer srv.Close()

	body := []byte(`{"jsonrpc":"1.0","id":7,"method":"getblockcount","params":[]}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var warm map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&warm); err != nil {
		t.Fatal(err)
	}
	errObj, _ := warm["error"].(map[string]interface{})
	if errObj == nil {
		t.Fatalf("expected warmup error, got %#v", warm)
	}
	if int(errObj["code"].(float64)) != rpcWarmupCode {
		t.Fatalf("code %v", errObj["code"])
	}

	j := &memJournal{tip: 12, best: "aa", gen: "bb", count: 13}
	es.Activate(HandlerCore("test", j, nil, nil, nil, nil, nil, true))

	res2, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	var out struct {
		Result int64 `json:"result"`
	}
	if err := json.NewDecoder(res2.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result != 12 {
		t.Fatalf("result %d", out.Result)
	}
}

func TestStartEarlyListenBinds(t *testing.T) {
	es, err := StartEarlyListen("127.0.0.1:0", httptls.Pair{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if es == nil {
		t.Fatal("nil early server")
	}
}
