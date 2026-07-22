// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "testing"

func TestRPCStatusLabel(t *testing.T) {
	if got := RPCStatusLabel("", false, false); got != "off" {
		t.Fatalf("empty addr: %q", got)
	}
	if got := RPCStatusLabel("127.0.0.1:22555", false, false); got != "starting" {
		t.Fatalf("starting: %q", got)
	}
	if got := RPCStatusLabel("127.0.0.1:22555", true, false); got != "warming_up" {
		t.Fatalf("warming: %q", got)
	}
	if got := RPCStatusLabel("127.0.0.1:22555", true, true); got != "ready" {
		t.Fatalf("ready: %q", got)
	}
}

func TestEnrichRPCSummaryFields(t *testing.T) {
	m := map[string]any{}
	EnrichRPCSummaryFields(m, "127.0.0.1:22555", func() (bool, bool) { return true, false })
	if m["rpc_status"] != "warming_up" {
		t.Fatalf("status %v", m["rpc_status"])
	}
	if m["rpc_listening"] != true {
		t.Fatalf("listening %v", m["rpc_listening"])
	}
}
