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

func TestApplyCoreRPCFormOverride(t *testing.T) {
	conf := config.File{CoreRPCAddr: "old:1"}
	conf = ApplyCoreRPCFormOverride(conf, "127.0.0.1:22555", "u", "p")
	if conf.CoreRPCAddr != "127.0.0.1:22555" || conf.CoreRPCUser != "u" || conf.CoreRPCPassword != "p" {
		t.Fatalf("override: %+v", conf)
	}
	conf = ApplyCoreRPCFormOverride(conf, "", "", "")
	if conf.CoreRPCAddr != "127.0.0.1:22555" {
		t.Fatalf("empty override should keep addr: %q", conf.CoreRPCAddr)
	}
}

func TestCoreRPCExplicitlyConfigured(t *testing.T) {
	if CoreRPCExplicitlyConfigured("mainnet", config.File{}) {
		t.Fatal("expected false for empty config")
	}
	if !CoreRPCExplicitlyConfigured("mainnet", config.File{CoreRPCAddr: "127.0.0.1:22555"}) {
		t.Fatal("expected true when addr set")
	}
}

func TestAnnotateCoreParitySummary(t *testing.T) {
	s := map[string]any{}
	AnnotateCoreParitySummary(s, "mainnet", config.File{CoreRPCAddr: "127.0.0.1:22555"})
	if s["core_rpc_addr"] != "127.0.0.1:22555" || s["core_rpc_configured"] != true {
		t.Fatalf("summary: %+v", s)
	}
}

func TestProbeCoreTestUnreachable(t *testing.T) {
	conf := config.File{CoreRPCAddr: "127.0.0.1:1"}
	res := ProbeCoreTest("mainnet", conf)
	if res.CoreAvailable || res.OK {
		t.Fatalf("expected unreachable core: %+v", res)
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected error message")
	}
}
