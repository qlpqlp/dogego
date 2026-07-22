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

	"dogego/consensus"
)

func TestExecEcho(t *testing.T) {
	params := []json.RawMessage{
		json.RawMessage(`"hello"`),
		json.RawMessage(`42`),
		json.RawMessage(`{"a":1}`),
	}
	out, code, msg := execEcho(params)
	if code != 0 {
		t.Fatalf("code=%d %s", code, msg)
	}
	if len(out) != 3 {
		t.Fatalf("len %d", len(out))
	}
	if out[0] != "hello" || out[1].(float64) != 42 {
		t.Fatalf("got %#v", out)
	}
	m, ok := out[2].(map[string]interface{})
	if !ok || m["a"].(float64) != 1 {
		t.Fatalf("obj %#v", out[2])
	}
}

func TestExecEchoEmpty(t *testing.T) {
	out, code, _ := execEcho(nil)
	if code != 0 || len(out) != 0 {
		t.Fatalf("got %#v code=%d", out, code)
	}
}

func TestHandlerEchojson(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"echojson","params":[true,["x"],"z"]}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []interface{} `json:"result"`
		Error  interface{}   `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("%+v", out.Error)
	}
	if len(out.Result) != 3 {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestExecEstimatePriority(t *testing.T) {
	v, code, msg := execEstimatePriority(nil, []json.RawMessage{json.RawMessage(`6`)})
	if code != 0 {
		t.Fatalf("%s", msg)
	}
	if v.(float64) != -1 {
		t.Fatalf("got %#v", v)
	}
	_, code, msg = execEstimatePriority(nil, nil)
	if code == 0 {
		t.Fatal("expected error")
	}
}

func TestExecEstimateSmartPriority(t *testing.T) {
	raw, code, msg := execEstimateSmartPriority(nil, []json.RawMessage{json.RawMessage(`12`)})
	if code != 0 {
		t.Fatalf("%s", msg)
	}
	m := raw.(map[string]interface{})
	if m["priority"].(float64) != consensus.InfPriority {
		t.Fatalf("%#v", m)
	}
	switch b := m["blocks"].(type) {
	case int:
		if b != 12 {
			t.Fatalf("blocks %d", b)
		}
	case float64:
		if b != 12 {
			t.Fatalf("blocks %v", b)
		}
	default:
		t.Fatalf("blocks type %T", m["blocks"])
	}
	raw2, _, _ := execEstimateSmartPriority(nil, nil)
	m2 := raw2.(map[string]interface{})
	switch b := m2["blocks"].(type) {
	case int:
		if b != 6 {
			t.Fatalf("blocks %d", b)
		}
	case float64:
		if b != 6 {
			t.Fatalf("blocks %v", b)
		}
	default:
		t.Fatalf("blocks type %T", m2["blocks"])
	}
}
