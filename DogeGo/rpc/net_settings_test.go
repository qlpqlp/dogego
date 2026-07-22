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
	"sync/atomic"
	"testing"
)

func TestExecSetMaxConnections(t *testing.T) {
	_, code, _ := execSetMaxConnections(nil, []json.RawMessage{json.RawMessage(`7`)})
	if code == 0 {
		t.Fatal("expected min 8 error")
	}
	_, code, _ = execSetMaxConnections(nil, []json.RawMessage{json.RawMessage(`33`)})
	if code == 0 {
		t.Fatal("expected max 32 error")
	}
	_, code, msg := execSetMaxConnections(nil, []json.RawMessage{json.RawMessage(`12`)})
	if code != CodeRPCP2PDisabled || msg != ErrP2PDisabled {
		t.Fatalf("p2p disabled code=%d msg=%q", code, msg)
	}
	var called atomic.Int32
	paths := &DataPaths{
		SetMaxConnections: func(max int) error {
			called.Store(int32(max))
			return nil
		},
	}
	ok, code, msg := execSetMaxConnections(paths, []json.RawMessage{json.RawMessage(`16`)})
	if code != 0 || !ok {
		t.Fatalf("code=%d msg=%s ok=%v", code, msg, ok)
	}
	if called.Load() != 16 {
		t.Fatalf("called %d", called.Load())
	}
}

func TestExecSetNetworkActive(t *testing.T) {
	var flag atomic.Bool
	flag.Store(true)
	paths := &DataPaths{
		NetworkActive: func() bool { return flag.Load() },
		SetNetworkActive: func(active bool) (bool, error) {
			flag.Store(active)
			return flag.Load(), nil
		},
	}
	v, code, msg := execSetNetworkActive(paths, []json.RawMessage{json.RawMessage(`false`)})
	if code != 0 || v {
		t.Fatalf("code=%d msg=%v want false", code, v)
	}
	if paths.NetworkActive() {
		t.Fatal("expected inactive")
	}
	_, code, msg = execSetNetworkActive(paths, []json.RawMessage{json.RawMessage(`true`)})
	if code != 0 || !paths.NetworkActive() {
		t.Fatalf("reactivate code=%d msg=%s", code, msg)
	}
}

func TestHandlerGetnetworkinfoReflectsNetworkActive(t *testing.T) {
	var flag atomic.Bool
	flag.Store(true)
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{
		NetworkActive: func() bool { return flag.Load() },
		SetNetworkActive: func(active bool) (bool, error) {
			flag.Store(active)
			return flag.Load(), nil
		},
	}
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"setnetworkactive","params":[false]}`)
	res, _ := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	var out1 map[string]interface{}
	_ = json.NewDecoder(res.Body).Decode(&out1)
	res.Body.Close()
	if out1["error"] != nil {
		t.Fatalf("%+v", out1["error"])
	}
	body2 := []byte(`{"jsonrpc":"1.0","id":2,"method":"getnetworkinfo","params":[]}`)
	res2, _ := http.Post(srv.URL, "application/json", bytes.NewReader(body2))
	var out2 map[string]interface{}
	_ = json.NewDecoder(res2.Body).Decode(&out2)
	res2.Body.Close()
	m := out2["result"].(map[string]interface{})
	if m["networkactive"].(bool) != false {
		t.Fatalf("networkactive %#v", m["networkactive"])
	}
	if m["connections"].(float64) != 0 {
		t.Fatalf("connections %#v", m["connections"])
	}
	if m["connections_in"].(float64) != 0 || m["connections_out"].(float64) != 0 {
		t.Fatalf("in/out %#v %#v", m["connections_in"], m["connections_out"])
	}
}

func TestHandlerGetnetworkinfoInactiveIgnoresP2PStats(t *testing.T) {
	var flag atomic.Bool
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{
		NetworkActive: func() bool { return flag.Load() },
		ConnectionCount: func() int { return 3 },
		P2PStats: func() map[string]any {
			return map[string]any{
				"connections_inbound":       1,
				"connections_outbound":      2,
				"connections_total":         5,
				"block_assist_connections":  2,
			}
		},
	}
	flag.Store(false)
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()
	res, _ := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getnetworkinfo","id":1}`)))
	var out map[string]interface{}
	_ = json.NewDecoder(res.Body).Decode(&out)
	res.Body.Close()
	m := out["result"].(map[string]interface{})
	if m["connections"].(float64) != 0 {
		t.Fatalf("connections %#v", m["connections"])
	}
	if m["connections_in"].(float64) != 0 {
		t.Fatalf("connections_in %#v", m["connections_in"])
	}
	if _, ok := m["dogego_block_assist_connections"]; ok {
		t.Fatalf("assist when inactive %#v", m["dogego_block_assist_connections"])
	}
}
