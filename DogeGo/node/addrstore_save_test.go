// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"path/filepath"
	"testing"
)

func TestMaybeSaveAddrBookIfDirty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "learned_addrs.json")
	b := NewAddrBook()
	b.AddSeen("93.184.216.90:22556")
	RecordOutboundDialTry(b, "93.184.216.90:22556")
	RecordOutboundHandshakeResult(b, "93.184.216.90:22556", nil)
	MaybeSaveAddrBookIfDirty(path, b)
	loaded, err := LoadAddrBook(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.IsTried("93.184.216.90:22556") {
		t.Fatal("probe success should persist tried entry")
	}
}

func TestActiveAddrBookPrefersPeerMgr(t *testing.T) {
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 2}, mustTestnetParams(t), "/DogeGo/", net.Dialer{})
	pm.SetAddrBook(NewAddrBook())
	pm.addrs.AddSeen("10.0.0.1:22556")
	bootstrap := NewAddrBook()
	bootstrap.AddSeen("8.8.8.8:22556")
	got := activeAddrBook(pm, bootstrap)
	if got != pm.addrs {
		t.Fatal("peer mgr book should win")
	}
	if activeAddrBook(nil, bootstrap) != bootstrap {
		t.Fatal("bootstrap when no mgr")
	}
}
