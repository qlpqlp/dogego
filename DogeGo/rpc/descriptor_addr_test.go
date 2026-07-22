// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/chain"
)

func TestParseAddrDescriptor(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	parsed, ok := parseWatchDescriptor("addr(" + addr + ")")
	if !ok || parsed.scriptType != "pkh" || parsed.addr != addr {
		t.Fatalf("parse %#v ok=%v", parsed, ok)
	}
	if parsed.normalized != "addr("+addr+")" {
		t.Fatalf("normalized %q", parsed.normalized)
	}
	_, ok = parseWatchDescriptor("addr(invalid!!!)")
	if ok {
		t.Fatal("expected invalid addr descriptor")
	}
}
