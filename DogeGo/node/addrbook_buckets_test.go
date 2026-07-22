// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"net"
	"path/filepath"
	"testing"
)

func TestAddrBookBucketAssignment(t *testing.T) {
	b := NewAddrBook()
	addr := "93.184.216.50:22556"
	b.AddSeen(addr)
	b.mu.Lock()
	rec := b.by[addr]
	nKey := b.nKey
	b.mu.Unlock()
	if rec == nil {
		t.Fatal("missing record")
	}
	if rec.TriedBucket < 0 || rec.TriedBucket >= addrTriedBucketCount {
		t.Fatalf("tried bucket %d out of range", rec.TriedBucket)
	}
	if rec.NewBucket < 0 || rec.NewBucket >= addrNewBucketCount {
		t.Fatalf("new bucket %d out of range", rec.NewBucket)
	}
	wantTried := triedBucketFor(nKey, addr)
	wantNew := newBucketFor(nKey, addr, rec.Group16, "")
	if rec.TriedBucket != wantTried || rec.NewBucket != wantNew {
		t.Fatalf("buckets tried=%d new=%d want tried=%d new=%d", rec.TriedBucket, rec.NewBucket, wantTried, wantNew)
	}
}

func TestAddrBookTriedCapMatchesCoreSlots(t *testing.T) {
	if maxAddrBookTried != addrTriedBucketCount*addrBucketSlotCap {
		t.Fatalf("tried cap %d want %d×%d", maxAddrBookTried, addrTriedBucketCount, addrBucketSlotCap)
	}
	if maxAddrBookNew != addrNewBucketCount*addrBucketSlotCap {
		t.Fatalf("new cap %d want %d×%d", maxAddrBookNew, addrNewBucketCount, addrBucketSlotCap)
	}
	if maxAddrBookTotal != maxAddrBookTried+maxAddrBookNew {
		t.Fatalf("total cap %d want tried+new", maxAddrBookTotal)
	}
	if maxLearnedAddrsOnDisk != maxAddrBookTotal {
		t.Fatalf("disk cap %d want %d", maxLearnedAddrsOnDisk, maxAddrBookTotal)
	}
}

func TestAddrBookTriedCapUsesBuckets(t *testing.T) {
	// Per-bucket 64-slot eviction is covered elsewhere; exercise modest tried growth under the global Core-scale cap.
	b := NewAddrBook()
	for i := 0; i < 400; i++ {
		addr := formatTestAddr(i)
		b.AddSeen(addr)
		b.NoteSuccess(addr)
	}
	tried, _ := b.AddrBookStats()
	if tried > maxAddrBookTried {
		t.Fatalf("tried %d > cap %d", tried, maxAddrBookTried)
	}
	if tried < 300 {
		t.Fatalf("tried %d; expected most of 400 to remain under Core-scale cap", tried)
	}
}

func TestAddrBookBucketStats(t *testing.T) {
	b := NewAddrBook()
	for i := 0; i < 20; i++ {
		b.AddSeen(formatTestAddr(i))
	}
	tbUsed, nbUsed, tbMax, nbMax := b.AddrBookBucketStats()
	if tbUsed != 0 {
		t.Fatalf("tried buckets used %d want 0", tbUsed)
	}
	if nbUsed <= 0 || nbUsed > 20 {
		t.Fatalf("new buckets used %d", nbUsed)
	}
	if nbMax <= 0 || nbMax > 20 {
		t.Fatalf("new bucket max fill %d", nbMax)
	}
	b.NoteSuccess(formatTestAddr(0))
	tbUsed, _, tbMax, _ = b.AddrBookBucketStats()
	if tbUsed != 1 || tbMax != 1 {
		t.Fatalf("after promote: tried buckets used=%d maxFill=%d", tbUsed, tbMax)
	}
}

func TestAddrBookAddrManInfo(t *testing.T) {
	b := NewAddrBook()
	b.AddSeen(formatTestAddr(1))
	info := b.AddrManInfo()
	all, _ := info["all"].(map[string]interface{})
	if all["total"] != 1 || all["new"] != 1 {
		t.Fatalf("all %v", all)
	}
}

func TestAddrBookBucket64SlotCap(t *testing.T) {
	b := NewAddrBook()
	addrs := addrsSharingNewBucket(t, b, addrBucketSlotCap+5)
	for i, addr := range addrs {
		b.AddSeenFrom(addr, 0, int64(1000+i), "")
	}
	_, _, _, nbMax := b.AddrBookBucketStats()
	if nbMax > addrBucketSlotCap {
		t.Fatalf("new bucket max fill %d > cap %d", nbMax, addrBucketSlotCap)
	}
	// Snapshot may exceed one bucket's slot cap when Group16 reassignment spreads peers.
	if len(b.Snapshot()) > maxAddrBookNew {
		t.Fatalf("snapshot len %d > new-table cap %d", len(b.Snapshot()), maxAddrBookNew)
	}
}

func TestAddrBookTriedBucket64SlotCap(t *testing.T) {
	b := NewAddrBook()
	addrs := addrsSharingTriedBucket(t, b, addrBucketSlotCap+5)
	for i, addr := range addrs {
		b.AddSeenFrom(addr, 0, int64(2000+i), "")
		b.NoteSuccess(addr)
	}
	_, _, tbMax, _ := b.AddrBookBucketStats()
	if tbMax > addrBucketSlotCap {
		t.Fatalf("tried bucket max fill %d > cap %d", tbMax, addrBucketSlotCap)
	}
}

func addrsSharingNewBucket(t *testing.T, b *AddrBook, need int) []string {
	t.Helper()
	if need <= 0 {
		t.Fatal("need > 0")
	}
	if b == nil {
		t.Fatal("nil book")
	}
	b.mu.Lock()
	nKey := b.nKey
	b.mu.Unlock()
	target := -1
	out := make([]string, 0, need)
	for i := 0; len(out) < need; i++ {
		addr := formatTestAddr(10000 + i)
		host := hostFromAddrPort(addr)
		g16 := addrGroup16(net.ParseIP(host))
		bk := newBucketFor(nKey, addr, g16, "")
		if target < 0 {
			target = bk
			out = append(out, addr)
			continue
		}
		if bk == target {
			out = append(out, addr)
		}
	}
	return out
}

func addrsSharingTriedBucket(t *testing.T, b *AddrBook, need int) []string {
	t.Helper()
	if need <= 0 {
		t.Fatal("need > 0")
	}
	if b == nil {
		t.Fatal("nil book")
	}
	b.mu.Lock()
	nKey := b.nKey
	b.mu.Unlock()
	target := -1
	out := make([]string, 0, need)
	for i := 0; len(out) < need; i++ {
		addr := formatTestAddr(i)
		bk := triedBucketFor(nKey, addr)
		if target < 0 {
			target = bk
			out = append(out, addr)
			continue
		}
		if bk == target {
			out = append(out, addr)
		}
	}
	return out
}

func formatTestAddr(i int) string {
	// Unique routable peers: 254 hosts per /16 (93.2.b.c), injective for test ranges.
	b := (i / 254) % 256
	c := 1 + (i % 254)
	return fmt.Sprintf("93.2.%d.%d:22556", b, c)
}

func assertAddrBookInvariants(t *testing.T, b *AddrBook) {
	t.Helper()
	tried, newAddrs := b.AddrBookStats()
	if tried > maxAddrBookTried {
		t.Fatalf("tried %d > cap %d", tried, maxAddrBookTried)
	}
	if newAddrs > maxAddrBookNew {
		t.Fatalf("new %d > cap %d", newAddrs, maxAddrBookNew)
	}
	if len(b.Snapshot()) > maxAddrBookTotal {
		t.Fatalf("snapshot len %d > total cap %d", len(b.Snapshot()), maxAddrBookTotal)
	}
	_, _, tbMax, nbMax := b.AddrBookBucketStats()
	if tbMax > addrBucketSlotCap || nbMax > addrBucketSlotCap {
		t.Fatalf("bucket fill triedMax=%d newMax=%d cap=%d", tbMax, nbMax, addrBucketSlotCap)
	}
}

// TestAddrBookChurnSoak exercises add/try/success/failure/prune/dial under load; caps must hold.
func TestAddrBookChurnSoak(t *testing.T) {
	b := NewAddrBook()
	now := int64(1_700_000_000)
	skip := map[string]struct{}{}
	for i := 0; i < 8000; i++ {
		addr := formatTestAddr(i % 3000)
		switch i % 8 {
		case 0, 1:
			seen := now + int64(i%3600)
			if i%16 == 0 {
				seen = now - addrStaleAfterSeconds - int64(i%1000)
			}
			b.AddSeenFrom(addr, 1, seen, "")
		case 2:
			b.NoteTry(addr)
		case 3:
			b.NoteSuccess(addr)
		case 4:
			b.NoteFailure(addr)
		case 5:
			_ = b.PickBest(skip, "", nil, nil)
		case 6:
			_ = b.PickFeeler(skip, "")
		default:
			_ = b.AddrSample(8, 1)
		}
		if i%200 == 0 {
			assertAddrBookInvariants(t, b)
		}
	}
	assertAddrBookInvariants(t, b)
}

func TestAddrBookSaveLoadPreservesBuckets(t *testing.T) {
	b := NewAddrBook()
	for i := 0; i < 120; i++ {
		addr := formatTestAddr(i)
		b.AddSeenFrom(addr, 1, int64(1_700_000_000+i), "")
		if i%3 == 0 {
			b.NoteSuccess(addr)
		}
	}
	path := filepath.Join(t.TempDir(), "learned_addrs.json")
	if err := SaveAddrBook(path, b); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadAddrBook(path)
	if err != nil {
		t.Fatal(err)
	}
	beforeTB, beforeNB, beforeTBMax, beforeNBMax := b.AddrBookBucketStats()
	afterTB, afterNB, afterTBMax, afterNBMax := loaded.AddrBookBucketStats()
	if beforeTB != afterTB || beforeNB != afterNB || beforeTBMax != afterTBMax || beforeNBMax != afterNBMax {
		t.Fatalf("bucket stats before=(%d,%d,%d,%d) after=(%d,%d,%d,%d)",
			beforeTB, beforeNB, beforeTBMax, beforeNBMax, afterTB, afterNB, afterTBMax, afterNBMax)
	}
	triedBefore, newBefore := b.AddrBookStats()
	triedAfter, newAfter := loaded.AddrBookStats()
	if triedBefore != triedAfter || newBefore != newAfter {
		t.Fatalf("stats before tried=%d new=%d after tried=%d new=%d", triedBefore, newBefore, triedAfter, newAfter)
	}
	assertAddrBookInvariants(t, loaded)
}

func TestAddrBookNKeyPersistRoundTrip(t *testing.T) {
	b := NewAddrBook()
	addr := "93.184.216.50:22556"
	b.AddSeen(addr)
	b.mu.Lock()
	nKeyBefore := b.nKey
	rec := b.by[addr]
	triedBefore := rec.TriedBucket
	newBefore := rec.NewBucket
	b.mu.Unlock()
	path := filepath.Join(t.TempDir(), "learned_addrs.json")
	if err := SaveAddrBook(path, b); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadAddrBook(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.HasAddrmanKey() {
		t.Fatal("nKey not loaded")
	}
	loaded.mu.Lock()
	nKeyAfter := loaded.nKey
	rec2 := loaded.by[addr]
	loaded.mu.Unlock()
	if nKeyBefore != nKeyAfter {
		t.Fatal("nKey changed on round trip")
	}
	if rec2 == nil || rec2.TriedBucket != triedBefore || rec2.NewBucket != newBefore {
		t.Fatalf("buckets changed tried=%d/%d new=%d/%d", triedBefore, rec2.TriedBucket, newBefore, rec2.NewBucket)
	}
}

func TestAddrBookTriedSlotAssigned(t *testing.T) {
	b := NewAddrBook()
	addr := "93.184.216.50:22556"
	b.AddSeen(addr)
	b.NoteSuccess(addr)
	b.mu.Lock()
	rec := b.by[addr]
	nKey := b.nKey
	b.mu.Unlock()
	if rec == nil || !rec.Tried {
		t.Fatal("expected tried record")
	}
	if len(rec.NewRefs) != 0 {
		t.Fatalf("tried should clear new refs, got %d", len(rec.NewRefs))
	}
	wantSlot := bucketPosition(nKey, false, rec.TriedBucket, addr)
	if rec.TriedSlot != wantSlot {
		t.Fatalf("tried slot %d want %d", rec.TriedSlot, wantSlot)
	}
}

func TestAddrBookNewMultiRef(t *testing.T) {
	b := NewAddrBook()
	addr := "93.184.216.50:22556"
	b.AddSeen(addr)
	b.mu.Lock()
	rec := b.by[addr]
	if rec == nil || len(rec.NewRefs) < 1 {
		b.mu.Unlock()
		t.Fatal("expected at least one new ref")
	}
	if rec.NewRefs[0].Slot < 0 || rec.NewRefs[0].Slot >= addrBucketSlotCap {
		b.mu.Unlock()
		t.Fatalf("bad slot %d", rec.NewRefs[0].Slot)
	}
	for i := 0; i < addrNewBucketsPerAddress+2; i++ {
		b.placeNewRefLocked(rec, formatTestAddr(1000+i), true)
	}
	n := len(rec.NewRefs)
	b.mu.Unlock()
	if n < 2 {
		t.Fatalf("expected multi-ref after forced place, got %d", n)
	}
	if n > addrNewBucketsPerAddress {
		t.Fatalf("refs %d > cap %d", n, addrNewBucketsPerAddress)
	}
}

func TestBucketPositionStable(t *testing.T) {
	var k addrmanKey
	copy(k[:], []byte("0123456789abcdef0123456789abcdef"))
	addr := "1.2.3.4:22556"
	a := bucketPosition(k, true, 42, addr)
	b := bucketPosition(k, true, 42, addr)
	if a != b || a < 0 || a >= addrBucketSlotCap {
		t.Fatalf("slot unstable or OOR: %d %d", a, b)
	}
	c := bucketPosition(k, false, 42, addr)
	if c == a {
		// unlikely but possible; only require in-range
	}
	if c < 0 || c >= addrBucketSlotCap {
		t.Fatalf("tried slot OOR %d", c)
	}
}
