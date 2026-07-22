// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigRequiresAuthWithRpcAllowIP(t *testing.T) {
	allow, err := ParseRPCAllowList([]string{"192.168.1.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	if !ConfigRequiresAuth("127.0.0.1:22557", allow) {
		t.Fatal("expected auth required when rpcallowip set")
	}
	if ConfigRequiresAuth("127.0.0.1:22557", nil) {
		t.Fatal("loopback bind without rpcallowip should not require auth config")
	}
}

func TestRejectRemoteWithoutAuth(t *testing.T) {
	h := wrapRejectRemoteWithoutAuth(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "203.0.113.1:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestValidateFilePathUnderDatadir(t *testing.T) {
	root := t.TempDir()
	child := root + "/wallet.dat"
	if _, err := ValidateFilePath([]string{root}, child, false); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateFilePath([]string{root}, "/etc/passwd", false); err == nil {
		t.Fatal("expected reject outside datadir")
	}
}
