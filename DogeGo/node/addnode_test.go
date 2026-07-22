// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"net"
	"testing"

	"dogego/chain"
)

func TestNormalizeNodeAddr(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("127.0.0.1:%d", p.Port)
	got, err := NormalizeNodeAddr("127.0.0.1", int(p.Port))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	_, _, err = net.SplitHostPort(got)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddedNodeStore(t *testing.T) {
	s := NewAddedNodeStore()
	s.Add("a:1")
	s.Add("b:2")
	if len(s.List()) != 2 {
		t.Fatalf("list %#v", s.List())
	}
	s.Remove("a:1")
	if len(s.List()) != 1 {
		t.Fatalf("after remove %#v", s.List())
	}
}
