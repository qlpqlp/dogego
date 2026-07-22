// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"path/filepath"
	"strings"
	"testing"

	"dogego/chain"
)

func TestAddressHDPathNodeTip(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := w.EnableNodeTip()
	if err != nil {
		t.Fatal(err)
	}
	path, chg, ok := w.AddressHDPath(addr)
	if !ok || chg {
		t.Fatalf("node tip path ok=%v chg=%v", ok, chg)
	}
	if !strings.Contains(path, "/2/0") {
		t.Fatalf("path %q", path)
	}
	if !w.IsNodeTipAddress(addr) {
		t.Fatal("expected IsNodeTipAddress")
	}
}
