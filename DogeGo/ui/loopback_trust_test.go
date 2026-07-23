// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"net/http"
	"testing"
)

func TestIsLoopbackTrustsPrivateWhenEnabled(t *testing.T) {
	t.Setenv("DOGEGO_TRUST_PRIVATE_CLIENTS", "")
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.RemoteAddr = "10.69.0.1:4321"
	if isLoopback(req) {
		t.Fatal("private client must not be trusted by default")
	}

	t.Setenv("DOGEGO_TRUST_PRIVATE_CLIENTS", "1")
	if !isLoopback(req) {
		t.Fatal("private client should be trusted when DOGEGO_TRUST_PRIVATE_CLIENTS=1")
	}

	req.RemoteAddr = "127.0.0.1:4321"
	if !isLoopback(req) {
		t.Fatal("loopback always trusted")
	}

	// Public / documentation IPs must never be treated as trusted, even with the opt-in.
	req.RemoteAddr = "203.0.113.1:1234"
	if isLoopback(req) {
		t.Fatal("public client must not be trusted")
	}
	req.RemoteAddr = "8.8.8.8:443"
	if isLoopback(req) {
		t.Fatal("public client must not be trusted")
	}

	// Other RFC1918 + link-local shapes Dogebox may use.
	for _, addr := range []string{
		"192.168.1.50:9",
		"172.16.0.2:9",
		"169.254.10.20:9",
		"[fd12:3456:789a::1]:9",
		"[fe80::1]:9",
	} {
		req.RemoteAddr = addr
		if !isLoopback(req) {
			t.Fatalf("want trusted private/link-local for %s", addr)
		}
	}
}

func TestTrustPrivateDashboardClientsEnv(t *testing.T) {
	t.Setenv("DOGEGO_TRUST_PRIVATE_CLIENTS", "")
	if trustPrivateDashboardClients() {
		t.Fatal("default off")
	}
	for _, v := range []string{"1", "true", "YES", "on"} {
		t.Setenv("DOGEGO_TRUST_PRIVATE_CLIENTS", v)
		if !trustPrivateDashboardClients() {
			t.Fatalf("want trust for %q", v)
		}
	}
	t.Setenv("DOGEGO_TRUST_PRIVATE_CLIENTS", "maybe")
	if trustPrivateDashboardClients() {
		t.Fatal("unknown value must be off")
	}
}
