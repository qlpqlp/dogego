// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"context"
	"encoding/json"
	"testing"
)

func TestScopedHostPermissions(t *testing.T) {
	dir := t.TempDir()
	chain := &ChainAdapter{NetworkName: "testnet"}
	m := NewManager(dir, "testnet", chain)
	man := Manifest{
		ID: "test.scope", Name: "scope", Version: "0.0.1",
		Permissions: []string{"datadir_write"},
		Entry:       Entry{Type: EntryWasm, Module: "test", Wasm: "x"},
	}
	host := m.hostFor(man.ID, man).(*scopedHost)

	if _, err := host.TipHeight(); err == nil {
		t.Fatal("expected chain_read denial")
	}
	if _, err := host.ExtensionDataDir("other.id"); err == nil {
		t.Fatal("expected own-id enforcement")
	}
	if _, err := host.ExtensionDataDir(man.ID); err != nil {
		t.Fatal(err)
	}

	man2 := Manifest{
		ID: "test.noperm", Name: "scope", Version: "0.0.1",
		Entry: Entry{Type: EntryWasm, Module: "test", Wasm: "x"},
	}
	host2 := m.hostFor(man2.ID, man2).(*scopedHost)
	if _, err := host2.ExtensionDataDir(man2.ID); err == nil {
		t.Fatal("expected datadir_write denial")
	}
}

func TestUnregisterPeerClearsOverlay(t *testing.T) {
	m := NewManager(t.TempDir(), "testnet", nil)
	send := func(string, []byte) error { return nil }
	m.NotifyPeerNegotiated("1.2.3.4:22556", []string{"zkproof-v1"}, send)
	if m.OverlayPeerCount("zkproof-v1") != 1 {
		t.Fatal("expected overlay peer")
	}
	m.UnregisterPeer("1.2.3.4:22556")
	if m.OverlayPeerCount("zkproof-v1") != 0 {
		t.Fatal("expected overlay cleared")
	}
	if len(m.PeerEnabledProtocols("1.2.3.4:22556")) != 0 {
		t.Fatal("expected negotiated protocols cleared")
	}
}

type permStubExt struct {
	man Manifest
}

func (s *permStubExt) Manifest() Manifest { return s.man }
func (s *permStubExt) OnEnable(context.Context, Host) error { return nil }
func (s *permStubExt) OnDisable() error                     { return nil }
func (s *permStubExt) HandleRPC(method string, _ []json.RawMessage, host Host) (interface{}, error) {
	if method != "chain" {
		return nil, nil
	}
	_, err := host.TipHeight()
	return nil, err
}

func TestHandleRPCUsesScopedHost(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, "testnet", &ChainAdapter{NetworkName: "testnet"})
	man := Manifest{
		ID: "test.rpc", Name: "rpc", Version: "0.0.1",
		Permissions: []string{"rpc_register"},
		Entry:       Entry{Type: EntryBuiltin, Module: "test"},
	}
	ext := &permStubExt{man: man}
	m.mu.Lock()
	m.active[man.ID] = ext
	m.activeManifest[man.ID] = man
	m.mu.Unlock()
	_, err := m.HandleRPC(FullRPCName(man.ID, "chain"), nil)
	if err == nil {
		t.Fatal("expected chain_read denial via scoped host")
	}
}
