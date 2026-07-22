// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"
)

func TestExecGetNodeAddressesCount(t *testing.T) {
	paths := &DataPaths{
		NodeAddresses: func(count int, network string) []map[string]interface{} {
			if count != 3 || network != "ipv4" {
				t.Fatalf("count=%d network=%q", count, network)
			}
			return []map[string]interface{}{
				{"address": "1.2.3.4", "port": 22556, "network": "ipv4"},
			}
		},
	}
	raw, _ := json.Marshal(3)
	net, _ := json.Marshal("ipv4")
	res, code, msg := execGetNodeAddresses(paths, []json.RawMessage{raw, net})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
	if len(res) != 1 {
		t.Fatalf("len %d", len(res))
	}
}

func TestExecGetNodeAddressesP2PDisabled(t *testing.T) {
	_, code, _ := execGetNodeAddresses(nil, nil)
	if code != CodeRPCP2PDisabled {
		t.Fatalf("code %d", code)
	}
}
