// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestRPCWhitelistNilAllowsAll(t *testing.T) {
	var w RPCWhitelist
	if !w.Allowed("sendrawtransaction") {
		t.Fatal("nil whitelist should allow")
	}
}

func TestRPCWhitelistRestricts(t *testing.T) {
	w := ParseRPCWhitelist([]string{"getblockcount", "getblockchaininfo"})
	if !w.Allowed("getblockcount") {
		t.Fatal("listed method")
	}
	if w.Allowed("sendrawtransaction") {
		t.Fatal("unlisted method")
	}
	if !w.Allowed("ping") {
		t.Fatal("ping always allowed")
	}
}
