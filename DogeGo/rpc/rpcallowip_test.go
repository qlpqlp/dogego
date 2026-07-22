// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRPCAllowListLoopbackAlways(t *testing.T) {
	a, err := ParseRPCAllowList(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Permits(netParse("127.0.0.1")) {
		t.Fatal("loopback")
	}
	if a.Permits(netParse("10.0.0.1")) {
		t.Fatal("expected deny")
	}
}

func TestRPCAllowListCIDR(t *testing.T) {
	a, err := ParseRPCAllowList([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	if !a.Permits(netParse("10.1.2.3")) {
		t.Fatal("inside CIDR")
	}
	if a.Permits(netParse("203.0.113.1")) {
		t.Fatal("outside CIDR")
	}
}

func netParse(s string) net.IP {
	return net.ParseIP(s)
}

func TestWrapRPCAllowIPForbidden(t *testing.T) {
	h := wrapRPCAllowIP(mustAllow(t, nil), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "203.0.113.7:12345"
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
}

func mustAllow(t *testing.T, specs []string) *RPCAllowList {
	t.Helper()
	a, err := ParseRPCAllowList(specs)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
