// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateManifestRejectsWalletPermission(t *testing.T) {
	m := Manifest{
		ManifestVersion: 1,
		ID:              "example.bad",
		Name:            "Bad",
		Version:         "0.0.1",
		Permissions:     []string{"wallet"},
		Entry:           Entry{Type: EntryWasm, Module: "x", Wasm: "mod.wasm"},
	}
	if err := ValidateManifest(m); err == nil {
		t.Fatal("expected wallet permission reject")
	}
}

func TestManagerListSubprocessZKL2(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, "testnet", nil)
	rows := m.List()
	for _, r := range rows {
		if r.ID == "dogego.zkl2" {
			t.Fatal("zkl2 should not be in List() before install")
		}
	}
	instDir := filepath.Join(dir, "extensions", "dogego.zkl2")
	if err := os.MkdirAll(instDir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("catalog", "zkl2", "dogego.extension.json"))
	if err != nil {
		t.Skip("catalog manifest missing")
	}
	if err := os.WriteFile(filepath.Join(instDir, "dogego.extension.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	rows = m.List()
	found := false
	for _, r := range rows {
		if r.ID == "dogego.zkl2" {
			found = true
			if !r.Installed || r.Builtin {
				t.Fatalf("zkl2 after install: %#v", r)
			}
			if r.EntryType != string(EntrySubprocess) {
				t.Fatalf("entry_type=%q want subprocess", r.EntryType)
			}
		}
	}
	if !found {
		t.Fatal("dogego.zkl2 not listed after install")
	}
}

type stubExt struct{ id string }

func (s *stubExt) Manifest() Manifest {
	return Manifest{ID: s.id, Name: "stub", Version: "0", Entry: Entry{Type: EntryBuiltin, Module: s.id}}
}
func (s *stubExt) OnEnable(ctx context.Context, host Host) error { return nil }
func (s *stubExt) OnDisable() error                              { return nil }
func (s *stubExt) HandleRPC(string, []json.RawMessage, Host) (interface{}, error) {
	return nil, nil
}
func (s *stubExt) RPCMethods() []RPCMethod {
	return []RPCMethod{{Name: "ping", Help: "stub ping"}}
}

func TestManagerCatalogRPCMethods(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, "testnet", nil)
	instDir := filepath.Join(dir, "extensions", "example.catalog")
	if err := os.MkdirAll(instDir, 0o755); err != nil {
		t.Fatal(err)
	}
	man := Manifest{
		ManifestVersion: ManifestVersion,
		ID:              "example.catalog",
		Name:            "catalog rpc test",
		Version:         "0.0.1",
		Entry:           Entry{Type: EntrySubprocess, Binary: "noop"},
		RPC:             []RPCMethod{{Name: "ping", Help: "stub ping"}},
	}
	raw, _ := json.Marshal(man)
	if err := os.WriteFile(filepath.Join(instDir, ManifestFileName), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	methods := m.CatalogRPCMethods()
	want := FullRPCName("example.catalog", "ping")
	found := false
	for _, name := range methods {
		if name == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("catalog missing %q: %v", want, methods)
	}
	h, ok := m.RPCHelp(want)
	if !ok || h == "" {
		t.Fatalf("RPCHelp %q ok=%v h=%q", want, ok, h)
	}
}

func TestInstallWasmZip(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, "testnet", nil)
	zipPath := filepath.Join("..", "docs", "extensions", "example-wasm", "ping.zip")
	if _, err := os.Stat(zipPath); err != nil {
		t.Skip("example wasm zip not built")
	}
	row, err := m.InstallZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	if row.ID != "example.wasm" {
		t.Fatalf("id %q", row.ID)
	}
	if err := m.Enable("example.wasm"); err != nil {
		t.Fatal(err)
	}
	out, err := m.HandleRPC(FullRPCName("example.wasm", "ping"), nil)
	if err != nil {
		t.Fatal(err)
	}
	ping, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("ping %#v", out)
	}
	if ping["pong"] != uint32(42) {
		t.Fatalf("want 42 got %#v", ping["pong"])
	}
}

func TestInstallZipManifest(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, "testnet", nil)
	zipPath := filepath.Join("catalog", "example-go", "dist", "hello.zip")
	if _, err := os.Stat(zipPath); err != nil {
		t.Skip("example zip not built")
	}
	row, err := m.InstallZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	if row.ID == "" {
		t.Fatal("empty id")
	}
}
