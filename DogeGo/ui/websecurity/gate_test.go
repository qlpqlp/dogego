// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package websecurity

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPINLockout(t *testing.T) {
	dir := t.TempDir()
	g, err := NewGate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.SetupPIN("", "123456"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxPINFailures; i++ {
		_, _ = g.UnlockPIN("000000")
	}
	st := g.Status(httptest.NewRequest("GET", "/", nil))
	if !st["locked"].(bool) {
		t.Fatalf("expected locked %#v", st)
	}
	_, err = g.UnlockPIN("123456")
	if err == nil {
		t.Fatal("expected locked error")
	}
}

func TestStatusWithoutPIN(t *testing.T) {
	dir := t.TempDir()
	g, err := NewGate(dir)
	if err != nil {
		t.Fatal(err)
	}
	st := g.Status(httptest.NewRequest("GET", "/", nil))
	if st["pin_enabled"].(bool) {
		t.Fatal("expected pin disabled")
	}
	if !st["unlocked"].(bool) {
		t.Fatalf("expected unlocked without PIN %#v", st)
	}
}

func TestUnlockSession(t *testing.T) {
	dir := t.TempDir()
	g, err := NewGate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.SetupPIN("", "654321"); err != nil {
		t.Fatal(err)
	}
	tok, err := g.UnlockPIN("654321")
	if err != nil || tok == "" {
		t.Fatalf("unlock: %v %q", err, tok)
	}
}

func TestLockClearsSession(t *testing.T) {
	dir := t.TempDir()
	g, err := NewGate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.SetupPIN("", "112233"); err != nil {
		t.Fatal(err)
	}
	tok, err := g.UnlockPIN("112233")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	st := g.Status(req)
	if !st["unlocked"].(bool) {
		t.Fatalf("expected unlocked with session %#v", st)
	}
	g.Lock()
	st = g.Status(req)
	if st["unlocked"].(bool) {
		t.Fatalf("expected locked after Lock() %#v", st)
	}
}
