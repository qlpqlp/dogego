// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"testing"
)

func TestRecordOutboundHandshakeResult(t *testing.T) {
	b := NewAddrBook()
	RecordOutboundDialTry(b, "93.184.216.80:22556")
	RecordOutboundHandshakeResult(b, "93.184.216.80:22556", nil)
	b.mu.Lock()
	rec := b.by["93.184.216.80:22556"]
	b.mu.Unlock()
	if rec == nil || !rec.Tried {
		t.Fatal("success should mark tried")
	}
	RecordOutboundDialTry(b, "93.184.216.81:22556")
	RecordOutboundHandshakeResult(b, "93.184.216.81:22556", errors.New("dial failed"))
	b.mu.Lock()
	rec2 := b.by["93.184.216.81:22556"]
	b.mu.Unlock()
	if rec2 == nil || rec2.Tried {
		t.Fatal("failure should not mark tried")
	}
	if rec2.Attempts < 1 {
		t.Fatal("failure should increment attempts")
	}
}
