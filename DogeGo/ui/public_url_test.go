// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"net"
	"testing"
)

type stubAddr string

func (s stubAddr) Network() string { return "tcp" }
func (s stubAddr) String() string  { return string(s) }

type stubListener struct {
	addr string
}

func (s stubListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (s stubListener) Close() error              { return nil }
func (s stubListener) Addr() net.Addr            { return stubAddr(s.addr) }

func TestPublicDashboardURLKeepsLocalhost(t *testing.T) {
	ln := stubListener{addr: "127.0.0.1:2013"}
	got := publicDashboardURL("https", "localhost:2013", ln)
	if got != "https://localhost:2013/" {
		t.Fatalf("got %q", got)
	}
}

func TestPublicDashboardURLKeepsConfiguredIP(t *testing.T) {
	ln := stubListener{addr: "127.0.0.1:2013"}
	got := publicDashboardURL("https", "127.0.0.1:2013", ln)
	if got != "https://127.0.0.1:2013/" {
		t.Fatalf("got %q", got)
	}
}

func TestPublicDashboardURLEphemeralPort(t *testing.T) {
	ln := stubListener{addr: "127.0.0.1:54321"}
	got := publicDashboardURL("http", "localhost:0", ln)
	if got != "http://localhost:54321/" {
		t.Fatalf("got %q", got)
	}
}
