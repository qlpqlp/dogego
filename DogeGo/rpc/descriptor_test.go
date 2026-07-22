// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"

	"dogego/chain"
)

func TestParseWatchDescriptorPKH(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr, _ := chain.RandomP2PKHAddress(p)
	desc := "pkh(" + addr + ")"
	parsed, ok := parseWatchDescriptor(desc)
	if !ok || parsed.addr != addr || parsed.scriptType != "pkh" {
		t.Fatalf("parse %#v ok=%v", parsed, ok)
	}
}

func TestExecGetDescriptorInfo(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr, _ := chain.RandomP2PKHAddress(p)
	raw, _ := json.Marshal("pkh(" + addr + ")")
	res, code, msg := execGetDescriptorInfo("testnet", nil, []json.RawMessage{raw})
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	m := res.(map[string]interface{})
	if m["checksum"] == "" || m["isrange"] != false {
		t.Fatalf("%#v", m)
	}
}
