// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
)

// Core addrman bucket counts (src/addrman.h): 256 tried Ã- 64 slots, 1024 new Ã- 64 slots.
const (
	addrTriedBucketCount = 256
	addrNewBucketCount   = 1024
	addrBucketSlotCap    = 64
)

func addrBucketHash(addr string) uint64 {
	sum := sha256.Sum256([]byte(addr))
	return binary.LittleEndian.Uint64(sum[:8])
}

func (b *AddrBook) assignBucketsLocked(r *AddrRecord, sourceHost string) {
	if b == nil || r == nil {
		return
	}
	assignAddrBucketsWithKey(b.nKey, r, sourceHost)
	if r.isTried() {
		b.placeTriedSlotLocked(r)
		return
	}
	if len(r.NewRefs) == 0 {
		b.placeNewRefLocked(r, sourceHost, true)
	} else {
		r.syncPrimaryNewBucket()
	}
}

func (b *AddrBook) backfillBucketsLocked() {
	for _, rec := range b.by {
		if rec == nil {
			continue
		}
		if rec.isTried() {
			if rec.TriedSlot < 0 || rec.TriedBucket < 0 {
				b.assignBucketsLocked(rec, "")
			}
			continue
		}
		if len(rec.NewRefs) == 0 {
			b.assignBucketsLocked(rec, "")
		} else {
			rec.syncPrimaryNewBucket()
		}
	}
}

// demoteTriedForCapLocked demotes one tried entry using Core-style bucket spread (fullest tried bucket, oldest LastSeen).
func (b *AddrBook) demoteTriedForCapLocked() {
	counts := make([]int, addrTriedBucketCount)
	var oldestByBucket [addrTriedBucketCount]struct {
		addr string
		seen int64
	}
	for addr, rec := range b.by {
		if rec == nil || !rec.isTried() {
			continue
		}
		if rec.TriedBucket < 0 || rec.TriedBucket >= addrTriedBucketCount {
			b.assignBucketsLocked(rec, "")
		}
		bk := rec.TriedBucket
		counts[bk]++
		o := oldestByBucket[bk]
		if o.addr == "" || rec.LastSeen < o.seen {
			oldestByBucket[bk] = struct {
				addr string
				seen int64
			}{addr: addr, seen: rec.LastSeen}
		}
	}
	targetBucket := -1
	maxCount := 0
	for i, n := range counts {
		if n > maxCount {
			maxCount = n
			targetBucket = i
		}
	}
	if targetBucket < 0 {
		return
	}
	drop := oldestByBucket[targetBucket].addr
	if drop == "" {
		return
	}
	rec := b.by[drop]
	if rec.TriedSlot >= 0 {
		b.clearTriedOccLocked(rec.TriedBucket, rec.TriedSlot, rec.Addr)
	}
	rec.Tried = false
	rec.TriedSlot = -1
	rec.Successes = 0
	rec.Attempts = 0
	b.placeNewRefLocked(rec, "", true)
}

func (b *AddrBook) removeAddrLocked(addr string) {
	if rec := b.by[addr]; rec != nil {
		b.clearAllOccForLocked(rec)
	}
	for i, a := range b.order {
		if a == addr {
			b.order = append(b.order[:i], b.order[i+1:]...)
			break
		}
	}
	delete(b.by, addr)
}

// trimNewBucketLocked removes oldest new-table slot refs until at most maxKeep remain in bucket.
func (b *AddrBook) trimNewBucketLocked(bucket, maxKeep int) {
	for guard := 0; guard < addrBucketSlotCap*4; guard++ {
		count := b.recountNewBucketLocked(bucket)
		if count <= maxKeep {
			return
		}
		oldest := ""
		var oldestSeen int64 = math.MaxInt64
		oldestSlot := -1
		for addr, rec := range b.by {
			if rec == nil || rec.isTried() {
				continue
			}
			for _, ref := range rec.NewRefs {
				if ref.Bucket != bucket {
					continue
				}
				if rec.LastSeen < oldestSeen {
					oldestSeen = rec.LastSeen
					oldest = addr
					oldestSlot = ref.Slot
				}
			}
		}
		if oldest == "" {
			return
		}
		rec := b.by[oldest]
		before := rec.newRefCount()
		b.removeNewRefLocked(rec, bucket, oldestSlot)
		if rec.newRefCount() == before {
			// Slot mismatch â€” drop all refs in this bucket for the addr.
			out := rec.NewRefs[:0]
			for _, ref := range rec.NewRefs {
				if ref.Bucket == bucket {
					continue
				}
				out = append(out, ref)
			}
			rec.NewRefs = out
			rec.syncPrimaryNewBucket()
		}
		if rec.newRefCount() == 0 {
			b.removeAddrLocked(oldest)
		}
	}
}

// trimTriedBucketLocked demotes oldest tried entries until at most maxKeep remain in bucket.
func (b *AddrBook) trimTriedBucketLocked(bucket, maxKeep int) {
	for {
		count := b.recountTriedBucketLocked(bucket)
		if count <= maxKeep {
			return
		}
		oldest := ""
		var oldestSeen int64 = math.MaxInt64
		for addr, rec := range b.by {
			if rec == nil || !rec.isTried() || rec.TriedBucket != bucket || rec.TriedSlot < 0 {
				continue
			}
			if rec.LastSeen < oldestSeen {
				oldestSeen = rec.LastSeen
				oldest = addr
			}
		}
		if oldest == "" {
			return
		}
		rec := b.by[oldest]
		if rec.TriedSlot >= 0 {
			b.clearTriedOccLocked(rec.TriedBucket, rec.TriedSlot, rec.Addr)
		}
		rec.Tried = false
		rec.TriedSlot = -1
		b.placeNewRefLocked(rec, "", true)
	}
}

func (b *AddrBook) enforceNewBucketSlotCapLocked(bucket int) {
	b.trimNewBucketLocked(bucket, addrBucketSlotCap-1)
}

func (b *AddrBook) enforceTriedBucketSlotCapLocked(bucket int) {
	b.trimTriedBucketLocked(bucket, addrBucketSlotCap-1)
}

func (b *AddrBook) enforceAllBucketSlotCapsLocked() {
	seenNew := make(map[int]struct{}, addrNewBucketCount)
	seenTried := make(map[int]struct{}, addrTriedBucketCount)
	for _, rec := range b.by {
		if rec == nil {
			continue
		}
		if rec.isTried() {
			if rec.TriedBucket < 0 || rec.TriedBucket >= addrTriedBucketCount || rec.TriedSlot < 0 {
				b.assignBucketsLocked(rec, "")
			}
			if rec.TriedBucket >= 0 {
				seenTried[rec.TriedBucket] = struct{}{}
			}
			continue
		}
		if len(rec.NewRefs) == 0 {
			b.assignBucketsLocked(rec, "")
		}
		for _, ref := range rec.NewRefs {
			seenNew[ref.Bucket] = struct{}{}
		}
		if rec.NewBucket >= 0 {
			seenNew[rec.NewBucket] = struct{}{}
		}
	}
	for bk := range seenNew {
		b.trimNewBucketLocked(bk, addrBucketSlotCap)
	}
	for bk := range seenTried {
		b.trimTriedBucketLocked(bk, addrBucketSlotCap)
	}
}
