// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package cgnat

import (
	"net"
	"testing"
)

func TestExternalIPLikelyCGNAT(t *testing.T) {
	if !externalIPLikelyCGNAT(net.ParseIP("100.64.1.2")) {
		t.Fatal("RFC6598")
	}
	if !externalIPLikelyCGNAT(net.ParseIP("192.168.1.1")) {
		t.Fatal("RFC1918")
	}
	if externalIPLikelyCGNAT(net.ParseIP("203.0.113.10")) {
		t.Fatal("public")
	}
}

func TestLikelyCgnatMode(t *testing.T) {
	if !Likely(nil, "cgnat", "auto", 22556) {
		t.Fatal("cgnat mode")
	}
}
