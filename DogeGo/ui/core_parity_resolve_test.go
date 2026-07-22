// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"testing"

	"dogego/config"
)

func TestResolveCoreParityEndpointsDefaults(t *testing.T) {
	ep := ResolveCoreParityEndpoints("mainnet", config.File{})
	if ep.Addr != "127.0.0.1:22555" {
		t.Fatalf("mainnet addr=%q", ep.Addr)
	}
	ep = ResolveCoreParityEndpoints("testnet", config.File{})
	if ep.Addr != "127.0.0.1:44556" {
		t.Fatalf("testnet addr=%q", ep.Addr)
	}
}

func TestResolveCoreParityEndpointsConfig(t *testing.T) {
	ep := ResolveCoreParityEndpoints("mainnet", config.File{
		CoreRPCAddr:     "127.0.0.1:9999",
		CoreRPCUser:     "coreuser",
		CoreRPCPassword: "corepass",
	})
	if ep.Addr != "127.0.0.1:9999" || ep.User != "coreuser" || ep.Pass != "corepass" {
		t.Fatalf("config override: %+v", ep)
	}
}

func TestRejectReasonMatches(t *testing.T) {
	if !rejectReasonMatches("bad-txns-inputs-duplicate", "bad-txns-inputs-duplicate") {
		t.Fatal("exact")
	}
	if !rejectReasonMatches("Missing inputs", "Missing") {
		t.Fatal("prefix")
	}
}
