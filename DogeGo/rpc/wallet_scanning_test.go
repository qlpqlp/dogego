// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestGetWalletInfoScanning(t *testing.T) {
	paths := &DataPaths{
		WalletAddress:    func() string { return "DAddr" },
		WalletIsScanning: func() bool { return true },
	}
	res, code, msg := execGetWalletInfo(paths, nil, nil, nil, nil, "test", nil)
	if code != 0 {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
	info, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("type %T", res)
	}
	scan, ok := info["scanning"].(map[string]interface{})
	if !ok {
		t.Fatalf("scanning %#v", info["scanning"])
	}
	if scan["duration"] != 0 {
		t.Fatalf("duration %#v", scan["duration"])
	}
}
