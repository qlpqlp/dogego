// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc_test

import (
	"testing"

	"dogego/rpc"
)

func TestLocalServiceNames(t *testing.T) {
	n := rpc.LocalServiceNames(1)
	if len(n) != 1 || n[0] != "NETWORK" {
		t.Fatalf("%#v", n)
	}
	n2 := rpc.LocalServiceNames(5)
	if len(n2) != 2 {
		t.Fatalf("%#v", n2)
	}
	n3 := rpc.LocalServiceNames(0)
	if len(n3) != 0 {
		t.Fatalf("%#v", n3)
	}
	if rpc.FormatServicesHex(1) != "0000000000000001" {
		t.Fatal(rpc.FormatServicesHex(1))
	}
}
