// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package websecurity

import (
	"net/http/httptest"
	"testing"
)

func TestRPFromRequest(t *testing.T) {
	r := httptest.NewRequest("GET", "http://127.0.0.1:2013/", nil)
	r.Host = "127.0.0.1:2013"
	rpID, origins := rpFromRequest(r)
	if rpID != "127.0.0.1" {
		t.Fatalf("rpID %q", rpID)
	}
	if len(origins) != 1 || origins[0] != "http://127.0.0.1:2013" {
		t.Fatalf("origins %v", origins)
	}

	r2 := httptest.NewRequest("GET", "http://localhost:2013/", nil)
	r2.Host = "localhost:2013"
	rpID, origins = rpFromRequest(r2)
	if rpID != "localhost" {
		t.Fatalf("localhost rpID %q", rpID)
	}
	if len(origins) != 1 || origins[0] != "http://localhost:2013" {
		t.Fatalf("localhost origins %v", origins)
	}
}
