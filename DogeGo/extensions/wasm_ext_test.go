// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWasmExtensionPing(t *testing.T) {
	dir := t.TempDir()
	man := Manifest{
		ID: "test.wasm", Name: "wasm", Version: "0.0.1",
		Permissions: []string{"rpc_register", "datadir_write"},
		Entry:       Entry{Type: EntryWasm, Module: "test", Wasm: "mod.wasm"},
	}
	if err := os.WriteFile(filepath.Join(dir, "mod.wasm"), TestPingWasm, 0o644); err != nil {
		t.Fatal(err)
	}
	ext, err := NewWasmExtension(dir, man)
	if err != nil {
		t.Fatal(err)
	}
	host := &testHost{dir: dir, id: man.ID}
	if err := ext.OnEnable(context.Background(), host); err != nil {
		t.Fatal(err)
	}
	defer ext.OnDisable()
	out, err := ext.HandleRPC("ping", nil, host)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("ping %#v", out)
	}
	if m["pong"] != uint32(42) {
		t.Fatalf("want 42 got %#v", m["pong"])
	}
}
