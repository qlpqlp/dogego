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

func TestBlockAssistPenalizeUsesAddrbook(t *testing.T) {
	scorer := NewBlockPeerScorer()
	book := NewAddrBook()
	addr := "93.184.216.55:22556"
	RecordOutboundHandshakeResult(book, addr, nil)
	penalizeBlockPeer(scorer, book, addr, true)
	book.mu.Lock()
	rec := book.by[addr]
	book.mu.Unlock()
	if rec == nil || rec.Attempts < 1 {
		t.Fatalf("hard assist failure should increment addrbook attempts, got %+v", rec)
	}
	if st, ok := scorer.Stats(addr); !ok || !st.InCooldown {
		t.Fatalf("scorer cooldown: %+v ok=%v", st, ok)
	}
}

func TestBlockAssistPenalizeNilBookStillScores(t *testing.T) {
	scorer := NewBlockPeerScorer()
	addr := "93.184.216.56:22556"
	penalizeBlockPeer(scorer, nil, addr, true)
	if st, ok := scorer.Stats(addr); !ok || !st.InCooldown {
		t.Fatalf("scorer should cooldown without addrbook")
	}
}

func TestRecordOutboundHandshakeFailure(t *testing.T) {
	b := NewAddrBook()
	RecordOutboundDialTry(b, "93.184.216.57:22556")
	RecordOutboundHandshakeResult(b, "93.184.216.57:22556", errors.New("reject"))
	b.mu.Lock()
	rec := b.by["93.184.216.57:22556"]
	b.mu.Unlock()
	if rec == nil || rec.Tried || rec.Attempts < 1 {
		t.Fatalf("handshake failure: %+v", rec)
	}
}
