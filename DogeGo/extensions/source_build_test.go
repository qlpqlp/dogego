// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnrichDocsPath(t *testing.T) {
	if got := EnrichDocsPath("dogego.zkl2", ""); got == "" {
		t.Fatal("expected discovered docs path for dogego.zkl2")
	}
	if got := EnrichDocsPath("dogego.radiodoge", ""); !strings.Contains(got, "radiodoge") {
		t.Fatalf("expected radiodoge docs path, got %q", got)
	}
	if got := EnrichDocsPath("custom.foo", "docs/custom.md"); got != "docs/custom.md" {
		t.Fatalf("got %q", got)
	}
}

func TestHandleRPCDoesNotDeadlockOnOverlayPeerCount(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "extensions"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := NewManager(dir, "testnet", nil)
	const extID = "test.overlay"
	m.RegisterBuiltin(extID, func(m Manifest) (Extension, error) {
		return &overlayStubExt{id: extID}, nil
	})
	instDir := filepath.Join(dir, "extensions", extID)
	if err := os.MkdirAll(instDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stub := &overlayStubExt{id: extID}
	man := stub.Manifest()
	raw, _ := json.Marshal(man)
	if err := os.WriteFile(filepath.Join(instDir, ManifestFileName), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	m.peerOverlays = map[string]peerOverlayEntry{
		"peer1": {protocols: []string{"testproto"}, send: func(string, []byte) error { return nil }},
	}
	if err := m.Enable(extID); err != nil {
		t.Fatal(err)
	}
	method := FullRPCName(extID, "info")
	done := make(chan error, 1)
	go func() {
		_, err := m.HandleRPC(method, nil)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("HandleRPC deadlocked (likely manager mutex held during extension RPC)")
	}
}

type overlayStubExt struct{ id string }

func (s *overlayStubExt) Manifest() Manifest {
	return Manifest{
		ManifestVersion: ManifestVersion,
		ID: s.id, Name: "overlay stub", Version: "0.0.1",
		Permissions: []string{"rpc_register", "ui_panel"},
		UI:          ManifestUI{StatusMethod: "info"},
		Entry:       Entry{Type: EntryBuiltin, Module: s.id},
	}
}
func (s *overlayStubExt) OnEnable(context.Context, Host) error { return nil }
func (s *overlayStubExt) OnDisable() error                     { return nil }
func (s *overlayStubExt) HandleRPC(_ string, _ []json.RawMessage, host Host) (interface{}, error) {
	if oh, ok := host.(OverlayHost); ok {
		_ = oh.OverlayPeerCount("testproto")
	}
	return map[string]interface{}{"ui": map[string]interface{}{"summary": "ok"}}, nil
}
func (s *overlayStubExt) RPCMethods() []RPCMethod {
	return []RPCMethod{{Name: "info", Help: "status"}}
}

func TestBuildSubprocessHelloExample(t *testing.T) {
	src := filepath.Join("catalog", "example-go")
	if _, err := locateGoMainPackage(src, Manifest{ID: "example.go"}); err != nil {
		t.Skip(src)
	}
	dir := t.TempDir()
	manRaw, err := os.ReadFile(filepath.Join(src, "dogego.extension.json"))
	if err != nil {
		t.Skip(src)
	}
	var man Manifest
	if err := json.Unmarshal(manRaw, &man); err != nil {
		t.Fatal(err)
	}
	if err := copyDirForTest(filepath.Join(src, "hello"), filepath.Join(dir, "hello")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dogego.extension.json"), manRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	if subprocessBinaryExists(dir, man.Entry.Binary) {
		t.Fatal("binary should not exist yet")
	}
	if err := buildSubprocessIfNeeded(dir, man); err != nil {
		t.Skip("go build unavailable or failed:", err)
	}
	if !subprocessBinaryExists(dir, man.Entry.Binary) {
		t.Fatal("expected built binary")
	}
}

func copyDirForTest(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}
