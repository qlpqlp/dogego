// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"encoding/json"
	"testing"
)

type stubWalletCaller struct {
	unlocked bool
	last     string
}

func (s *stubWalletCaller) WalletUnlocked() bool { return s.unlocked }

func (s *stubWalletCaller) Call(method string, params []json.RawMessage) (interface{}, error) {
	s.last = method
	if method == "signmessage" {
		return "stub-sig", nil
	}
	return map[string]interface{}{"ok": true}, nil
}

func TestValidateManifestAcceptsWalletRPCPermission(t *testing.T) {
	m := Manifest{
		ManifestVersion: 1,
		ID:              "com.example.wallet",
		Name:            "Wallet demo",
		Version:         "0.0.1",
		Permissions:     []string{"wallet_rpc", "rpc_register"},
		Entry:           Entry{Type: EntryWasm, Module: "x", Wasm: "mod.wasm"},
	}
	if err := ValidateManifest(m); err != nil {
		t.Fatal(err)
	}
}

func TestCallWalletRPCRequiresUnlockForSign(t *testing.T) {
	c := &stubWalletCaller{unlocked: false}
	_, err := CallWalletRPC(c, "signmessage", nil)
	if err == nil {
		t.Fatal("expected unlock error")
	}
	c.unlocked = true
	sig, err := CallWalletRPC(c, "signmessage", nil)
	if err != nil || sig != "stub-sig" {
		t.Fatalf("signmessage: err=%v sig=%#v", err, sig)
	}
}

func TestCallWalletRPCRejectsForbidden(t *testing.T) {
	c := &stubWalletCaller{unlocked: true}
	_, err := CallWalletRPC(c, "dumpprivkey", nil)
	if err == nil {
		t.Fatal("expected forbidden")
	}
}

func TestCallWalletRPCReadOnlyWithoutUnlock(t *testing.T) {
	c := &stubWalletCaller{unlocked: false}
	out, err := CallWalletRPC(c, "getwalletinfo", nil)
	if err != nil || out == nil {
		t.Fatalf("getwalletinfo: err=%v out=%#v", err, out)
	}
}

func TestScopedHostCallWalletRPCRequiresPermission(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, "testnet", nil)
	m.SetWalletRPC(&stubWalletCaller{unlocked: true})
	man := Manifest{
		ManifestVersion: 1,
		ID:              "com.example.noperms",
		Name:            "x",
		Version:         "0.0.1",
		Entry:           Entry{Type: EntryBuiltin, Module: "com.example.noperms"},
		Permissions:     []string{"rpc_register"},
	}
	host := m.hostFor("com.example.noperms", man)
	wh, ok := host.(WalletRPCHost)
	if !ok {
		t.Fatal("scoped host should implement WalletRPCHost")
	}
	_, err := wh.CallWalletRPC("getwalletinfo", nil)
	if err == nil {
		t.Fatal("expected permission error")
	}
}

func TestScopedHostCallWalletRPCWithPermission(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, "testnet", nil)
	m.SetWalletRPC(&stubWalletCaller{unlocked: true})
	man := Manifest{
		ManifestVersion: 1,
		ID:              "com.example.wallet",
		Name:            "x",
		Version:         "0.0.1",
		Entry:           Entry{Type: EntryBuiltin, Module: "com.example.wallet"},
		Permissions:     []string{"wallet_rpc"},
	}
	host := m.hostFor("com.example.wallet", man)
	wh := host.(WalletRPCHost)
	out, err := wh.CallWalletRPC("getwalletinfo", nil)
	if err != nil || out == nil {
		t.Fatalf("getwalletinfo: err=%v out=%#v", err, out)
	}
}
