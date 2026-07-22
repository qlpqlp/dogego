// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"

	"dogego/zmqnotify"
)

func TestExecGetZMQNotificationsEmpty(t *testing.T) {
	res, code, msg := execGetZMQNotifications(nil, nil)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 0 {
		t.Fatalf("want empty array, got %#v", res)
	}
}

func TestExecGetZMQNotificationsWired(t *testing.T) {
	cfg := zmqnotify.Config{
		PubHashBlock: "127.0.0.1:28332",
		PubHashTx:    "tcp://127.0.0.1:28333",
	}
	paths := &DataPaths{
		ZmqNotifications: func() []map[string]interface{} {
			if cfg.Enabled() {
				return cfg.ActiveNotifications()
			}
			return nil
		},
	}
	res, code, msg := execGetZMQNotifications(paths, nil)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 2 {
		t.Fatalf("want 2 rows, got %#v", res)
	}
	first, ok := arr[0].(map[string]interface{})
	if !ok || first["type"] != "pubhashblock" || first["address"] != "tcp://127.0.0.1:28332" {
		t.Fatalf("hashblock row=%#v", first)
	}
	if first["hwm"] != 1000 {
		t.Fatalf("hwm=%v want 1000", first["hwm"])
	}
	_, code, msg = execGetZMQNotifications(paths, []json.RawMessage{json.RawMessage(`0`)})
	if code != -32602 {
		t.Fatalf("extra param code=%d msg=%q", code, msg)
	}
}
