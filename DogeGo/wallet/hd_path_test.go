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

func TestAddressHDPath(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	recv, err := w.NewReceiveAddress()
	if err != nil {
		t.Fatal(err)
	}
	path, chg, ok := w.AddressHDPath(recv)
	if !ok || chg {
		t.Fatalf("recv path ok=%v chg=%v", ok, chg)
	}
	if !strings.HasSuffix(path, "/0/1") {
		t.Fatalf("path %q", path)
	}
}
