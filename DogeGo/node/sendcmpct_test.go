// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"testing"

	"dogego/chain"
	"dogego/wire"
)

func TestReplySendCmpctDecline(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	mw := NewMsgWriter(c1, p.Magic)
	body, err := wire.EncodeSendCmpct(true, 1)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_, pl, _ := wire.ReadMessage(c2, p.Magic)
		if len(pl) != 9 {
			t.Errorf("reply len %d", len(pl))
		}
	}()
	announce, err := ReplySendCmpctDecline(mw, body)
	if err != nil || !announce {
		t.Fatalf("announce %v err %v", announce, err)
	}
}
