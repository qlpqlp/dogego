// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSubprocessExtensionPing(t *testing.T) {
	bin := buildHelloTestBinary(t)
	dir := t.TempDir()
	man := Manifest{
		ID: "test.hello", Name: "test", Version: "0.0.1",
		Permissions: []string{"rpc_register", "datadir_write"},
		Entry:       Entry{Type: EntrySubprocess, Binary: filepath.Base(bin)},
		RPC: []RPCMethod{
			{Name: "info", Help: "info"},
			{Name: "ping", Help: "ping"},
		},
	}
	// copy binary into ext dir with expected name
	dest := filepath.Join(dir, filepath.Base(bin))
	raw, _ := os.ReadFile(bin)
	if err := os.WriteFile(dest, raw, 0o755); err != nil {
		t.Fatal(err)
	}
	ext, err := NewSubprocessExtension(dir, man)
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
	if out != "pong" {
		t.Fatalf("ping %#v", out)
	}
}

func TestSubprocessExtensionHelloWorld(t *testing.T) {
	bin := buildHelloTestBinary(t)
	dir := t.TempDir()
	man := Manifest{
		ID: "test.hello", Name: "test", Version: "0.2.0",
		Permissions: []string{"rpc_register", "datadir_write", "ui_panel", "chain_read"},
		Entry:       Entry{Type: EntrySubprocess, Binary: filepath.Base(bin)},
		RPC: []RPCMethod{
			{Name: "info", Help: "info"},
			{Name: "greet", Help: "greet"},
			{Name: "chain_tip", Help: "chain tip"},
		},
	}
	dest := filepath.Join(dir, filepath.Base(bin))
	raw, _ := os.ReadFile(bin)
	if err := os.WriteFile(dest, raw, 0o755); err != nil {
		t.Fatal(err)
	}
	ext, err := NewSubprocessExtension(dir, man)
	if err != nil {
		t.Fatal(err)
	}
	host := &testHost{dir: dir, id: man.ID, tip: 12345}
	if err := ext.OnEnable(context.Background(), host); err != nil {
		t.Fatal(err)
	}
	defer ext.OnDisable()

	greetRaw, _ := json.Marshal("Doge")
	out, err := ext.HandleRPC("greet", []json.RawMessage{greetRaw}, host)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fmt.Sprint(out), "Doge") {
		t.Fatalf("greet %#v", out)
	}

	info, err := ext.HandleRPC("info", nil, host)
	if err != nil {
		t.Fatal(err)
	}
	im, ok := info.(map[string]interface{})
	if !ok || im["ui"] == nil {
		t.Fatalf("info missing ui %#v", info)
	}

	tip, err := ext.HandleRPC("chain_tip", nil, host)
	if err != nil {
		t.Fatal(err)
	}
	tm, ok := tip.(map[string]interface{})
	if !ok || tm["tip_height"] != float64(12345) && tm["tip_height"] != int64(12345) {
		t.Fatalf("chain_tip %#v", tip)
	}
}

func buildHelloTestBinary(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "docs", "extensions", "example", "hello", "main.go")
	if _, err := os.Stat(src); err != nil {
		t.Skip("hello example source missing")
	}
	out := filepath.Join(t.TempDir(), "hello-ext")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", out, src)
	if out2, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build hello: %v %s", err, out2)
	}
	return out
}

type testHost struct {
	dir string
	id  string
	tip int64
}

func (h *testHost) Network() string                            { return "testnet" }
func (h *testHost) TipHeight() (int64, error)                  { return h.tip, nil }
func (h *testHost) GetRawBlockByHeight(int64) ([]byte, error)  { return nil, nil }
func (h *testHost) LookupTxHex(string) (string, int64, bool)   { return "", 0, false }
func (h *testHost) BlockHashAtHeight(int64) (string, error)    { return "", nil }
func (h *testHost) ConfirmedTxInBlock(string, string) (uint32, bool) { return 0, false }
func (h *testHost) DataDir() string                            { return h.dir }
func (h *testHost) ExtensionDataDir(id string) (string, error) { return filepath.Join(h.dir, id), nil }
func (h *testHost) Log(string)                                   {}
