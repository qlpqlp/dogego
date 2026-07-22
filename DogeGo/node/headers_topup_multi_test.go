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

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestRelayAddrsOrdered_excludesPrimary(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 4}, p, "test", net.Dialer{})
	c1, c2 := &fakeConn{}, &fakeConn{}
	pm.RegisterPrimary("primary:1", c1, NewMsgWriter(c1, p.Magic), nil, nil)
	pm.mu.Lock()
	pm.sessions["relay:1"] = &peerLink{addr: "relay:1", mw: NewMsgWriter(c2, p.Magic)}
	pm.order = append(pm.order, "relay:1")
	pm.mu.Unlock()
	addrs := pm.RelayAddrsOrdered(3)
	for _, a := range addrs {
		if a == "primary:1" {
			t.Fatalf("primary should not be in relay list: %v", addrs)
		}
	}
	if len(addrs) != 1 || addrs[0] != "relay:1" {
		t.Fatalf("got relay addrs %v", addrs)
	}
}

func TestEncodeTopUpGetHeaders(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "h.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	pl, err := encodeTopUpGetHeaders(j, p)
	if err != nil || len(pl) < 36 {
		t.Fatalf("payload len=%d err=%v", len(pl), err)
	}
}

type fakeConn struct {
	net.Conn
}

func (fakeConn) Read(b []byte) (int, error)  { return 0, nil }
func (fakeConn) Write(b []byte) (int, error) { return len(b), nil }
func (fakeConn) Close() error               { return nil }
