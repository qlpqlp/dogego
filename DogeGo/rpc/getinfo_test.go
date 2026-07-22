// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestExecGetInfoIBDFields(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(t.TempDir(), "h.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	res, code, msg := execGetInfo("test", j, nil, nil)
	if code != 0 {
		t.Fatalf("getinfo: %s", msg)
	}
	if _, ok := res["initialblockdownload"].(bool); !ok {
		t.Fatalf("initialblockdownload %#v", res["initialblockdownload"])
	}
	if _, ok := res["headers"].(int64); !ok {
		t.Fatalf("headers %#v", res["headers"])
	}
	if _, ok := res["verificationprogress"].(float64); !ok {
		t.Fatalf("verificationprogress %#v", res["verificationprogress"])
	}
	if active, ok := res["dogego_wallet_active"].(bool); !ok || active {
		t.Fatalf("dogego_wallet_active %#v", res["dogego_wallet_active"])
	}
}
