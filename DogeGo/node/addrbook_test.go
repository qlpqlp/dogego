// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"testing"
	"time"

	"dogego/chain"
)

func TestAddrBookPickBestCooldown(t *testing.T) {
	b := NewAddrBook()
	b.AddSeen("93.184.216.1:22556")
	b.NoteTry("93.184.216.1:22556")
	b.NoteFailure("93.184.216.1:22556")
	b.AddSeen("93.184.216.2:22556")
	skip := map[string]struct{}{}
	if got := b.PickBest(skip, "", nil, nil); got != "93.184.216.2:22556" {
		t.Fatalf("pick got %q want 93.184.216.2:22556", got)
	}
}

func TestAddrBookPickBestPrefersSuccess(t *testing.T) {
	b := NewAddrBook()
	b.AddSeen("93.184.216.10:22556")
	b.AddSeen("93.184.216.11:22556")
	b.NoteTry("93.184.216.10:22556")
	b.NoteSuccess("93.184.216.10:22556")
	b.NoteTry("93.184.216.11:22556")
	b.NoteFailure("93.184.216.11:22556")
	if got := b.PickBest(nil, "", nil, nil); got != "93.184.216.10:22556" {
		t.Fatalf("got %q", got)
	}
}

func TestAddrRecordFeelerOK(t *testing.T) {
	r := &AddrRecord{LastTry: time.Now().Unix(), Attempts: 2, Successes: 0}
	if r.feelerOK(time.Now()) {
		t.Fatal("should be in failure cooldown")
	}
	r2 := &AddrRecord{LastTry: time.Now().Add(-10 * time.Minute).Unix(), Attempts: 1, Successes: 1}
	if !r2.feelerOK(time.Now()) {
		t.Fatal("successful peer should be feeler-ok")
	}
}

func TestAddrBookPickBestPrefersTried(t *testing.T) {
	b := NewAddrBook()
	b.AddSeen("93.184.216.20:22556")
	b.AddSeen("93.184.216.21:22556")
	b.NoteTry("93.184.216.21:22556")
	b.NoteSuccess("93.184.216.21:22556")
	b.NoteTry("93.184.216.20:22556")
	if got := b.PickBest(nil, "", nil, nil); got != "93.184.216.21:22556" {
		t.Fatalf("got %q want 93.184.216.21:22556", got)
	}
}

func TestAddrBookPickFeelerSkipsTried(t *testing.T) {
	b := NewAddrBook()
	b.AddSeen("93.184.216.30:22556")
	b.NoteTry("93.184.216.30:22556")
	b.NoteSuccess("93.184.216.30:22556")
	b.AddSeen("93.184.216.31:22556")
	if got := b.PickFeeler(nil, ""); got != "93.184.216.31:22556" {
		t.Fatalf("got %q want 93.184.216.31:22556", got)
	}
}

func TestAddrBookDialScorePrefersNodeNetwork(t *testing.T) {
	b := NewAddrBook()
	b.AddSeenMeta("93.184.216.40:22556", chain.NodeNetwork, 0)
	b.AddSeen("93.184.216.41:22556")
	if got := b.PickBest(nil, "", nil, nil); got != "93.184.216.40:22556" {
		t.Fatalf("got %q want 93.184.216.40:22556", got)
	}
}

func TestAddrBookTriedCapDemotes(t *testing.T) {
	b := NewAddrBook()
	for i := 0; i < 400; i++ {
		addr := formatTestAddr(i)
		b.AddSeen(addr)
		b.NoteSuccess(addr)
	}
	tried, _ := b.AddrBookStats()
	if tried > maxAddrBookTried {
		t.Fatalf("stats tried %d > cap %d", tried, maxAddrBookTried)
	}
}

func TestAddrBookAddrSampleMix(t *testing.T) {
	b := NewAddrBook()
	b.AddSeenMeta("93.184.216.2:22556", chain.NodeNetwork, 0)
	b.AddSeen("93.184.216.1:22556")
	b.NoteTry("93.184.216.1:22556")
	b.NoteSuccess("93.184.216.1:22556")
	out := b.AddrSample(2, chain.NodeNetwork)
	if len(out) != 2 {
		t.Fatalf("sample len %d", len(out))
	}
	if out[0].HostPort() != "93.184.216.1:22556" {
		t.Fatalf("first %v want 93.184.216.1:22556", out[0].HostPort())
	}
}

func TestAddrBookSkipsNonRoutable(t *testing.T) {
	b := NewAddrBook()
	b.AddSeen("127.0.0.1:22556")
	b.AddSeen("1.1.1.1:22556")
	if got := b.PickBest(nil, "", nil, nil); got != "1.1.1.1:22556" {
		t.Fatalf("got %q", got)
	}
}

func TestAddrBookGroup16Spread(t *testing.T) {
	b := NewAddrBook()
	for i := 0; i < 5; i++ {
		b.AddSeenFrom(fmt.Sprintf("9.9.0.%d:22556", i+1), 0, 0, "")
	}
	b.AddSeenFrom("8.8.4.4:22556", 0, 0, "")
	if got := b.PickBest(nil, "", nil, nil); got != "8.8.4.4:22556" {
		t.Fatalf("prefer underrepresented /16, got %q", got)
	}
}

func TestAddrBookGroup16SpreadTried(t *testing.T) {
	b := NewAddrBook()
	for i := 0; i < 4; i++ {
		addr := fmt.Sprintf("9.8.0.%d:22556", i+1)
		b.AddSeen(addr)
		b.NoteSuccess(addr)
	}
	b.AddSeen("8.8.9.9:22556")
	b.NoteSuccess("8.8.9.9:22556")
	if got := b.PickBest(nil, "", nil, nil); got != "8.8.9.9:22556" {
		t.Fatalf("tried table spread: got %q want 8.8.9.9:22556", got)
	}
}

func TestPickBestSkipsConnectedGroup(t *testing.T) {
	b := NewAddrBook()
	b.AddSeen("9.7.0.1:22556")
	b.NoteSuccess("9.7.0.1:22556")
	b.AddSeen("9.7.0.2:22556")
	b.AddSeen("8.8.7.7:22556")
	skip := map[string]struct{}{"9.7.0.1:22556": {}}
	if got := b.PickBest(skip, "", nil, nil); got != "8.8.7.7:22556" {
		t.Fatalf("got %q want 8.8.7.7:22556 (avoid /16 of connected peer)", got)
	}
}

func TestAddrBookLoadSkipsNonRoutable(t *testing.T) {
	b := NewAddrBook()
	now := time.Now().Unix()
	b.loadRecords([]AddrRecord{
		{Addr: "127.0.0.1:22556", LastSeen: now},
		{Addr: "8.8.8.8:22556", LastSeen: now},
	})
	if len(b.Snapshot()) != 1 || b.Snapshot()[0] != "8.8.8.8:22556" {
		t.Fatalf("snapshot %v", b.Snapshot())
	}
}

func TestAddrBookClampsFutureSeenTime(t *testing.T) {
	b := NewAddrBook()
	now := time.Now().Unix()
	b.AddSeenFrom("8.8.4.4:22556", 0, now+addrMaxFutureOffsetSec+99, "")
	b.mu.Lock()
	rec := b.by["8.8.4.4:22556"]
	b.mu.Unlock()
	if rec == nil || rec.LastSeen != now {
		t.Fatalf("LastSeen %d want %d", rec.LastSeen, now)
	}
}

func TestEnforceTriedCapDemotesOne(t *testing.T) {
	now := time.Now().Unix()
	recs := make([]AddrRecord, maxAddrBookTried+1)
	for i := range recs {
		recs[i] = AddrRecord{Addr: formatTestAddr(i), LastSeen: now + int64(i), Tried: true, Successes: 1}
	}
	b := NewAddrBook()
	b.loadRecords(recs)
	tried, _ := b.AddrBookStats()
	if tried != maxAddrBookTried {
		t.Fatalf("tried count %d want %d", tried, maxAddrBookTried)
	}
	demoted := 0
	for _, rec := range b.by {
		if rec != nil && !rec.Tried {
			demoted++
		}
	}
	if demoted != 1 {
		t.Fatalf("demoted %d want 1", demoted)
	}
}

func TestAddrBookPickBestPrefersAddnode(t *testing.T) {
	b := NewAddrBook()
	b.AddSeen("93.184.216.50:22556")
	b.NoteTry("93.184.216.50:22556")
	b.NoteSuccess("93.184.216.50:22556")
	b.AddSeen("93.184.216.51:22556")
	if got := b.PickBest(nil, "", nil, []string{"93.184.216.51:22556"}); got != "93.184.216.51:22556" {
		t.Fatalf("got %q want 93.184.216.51:22556", got)
	}
}

func TestAddrBookAddrSampleGroupSpread(t *testing.T) {
	b := NewAddrBook()
	for i := 0; i < 4; i++ {
		b.AddSeenFrom(fmt.Sprintf("9.%d.1.1:22556", i+1), chain.NodeNetwork, 0, "")
	}
	b.AddSeenFrom("8.8.8.8:22556", chain.NodeNetwork, 0, "")
	out := b.AddrSample(3, chain.NodeNetwork)
	if len(out) < 2 {
		t.Fatalf("sample len %d", len(out))
	}
	groups := make(map[string]int)
	for _, na := range out {
		groups[addrGroup16(na.IP)]++
	}
	for g, n := range groups {
		if n > 1 {
			t.Fatalf("duplicate /16 %s in sample (%d addrs)", g, n)
		}
	}
}

func TestAddrBookTouchSeen(t *testing.T) {
	b := NewAddrBook()
	b.AddSeen("93.184.216.70:22556")
	b.mu.Lock()
	b.by["93.184.216.70:22556"].LastSeen = 1
	b.mu.Unlock()
	b.TouchSeen("93.184.216.70:22556")
	b.mu.Lock()
	seen := b.by["93.184.216.70:22556"].LastSeen
	b.mu.Unlock()
	if seen <= 1 {
		t.Fatalf("LastSeen %d not refreshed", seen)
	}
}

func TestNoteAddnodePersistentMarksTried(t *testing.T) {
	b := NewAddrBook()
	b.AddSeen("93.184.216.60:22556")
	b.NoteAddnodePersistent("93.184.216.60:22556")
	b.mu.Lock()
	rec := b.by["93.184.216.60:22556"]
	b.mu.Unlock()
	if rec == nil || !rec.Tried {
		t.Fatal("addnode should be in tried table before handshake")
	}
	if got := b.PickBest(nil, "", nil, nil); got != "93.184.216.60:22556" {
		t.Fatalf("tried addnode pick got %q", got)
	}
}
