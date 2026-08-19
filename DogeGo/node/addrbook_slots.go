// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/binary"
	"strconv"
)

// Core addrman.h: ADDRMAN_NEW_BUCKETS_PER_ADDRESS.
const addrNewBucketsPerAddress = 8

// AddrSlotRef is one vvNew[bucket][slot] occupancy for an address.
type AddrSlotRef struct {
	Bucket int `json:"b"`
	Slot   int `json:"s"`
}

// bucketPosition matches Core AddrInfo::GetBucketPosition (nKey + N/K + bucket + addr key).
func bucketPosition(nKey addrmanKey, fNew bool, bucket int, addr string) int {
	tag := byte('K')
	if fNew {
		tag = 'N'
	}
	host := hostFromAddrPort(addr)
	key := addrNetKey(host)
	var bbuf [4]byte
	binary.LittleEndian.PutUint32(bbuf[:], uint32(uint(bucket)))
	if key == nil {
		sum := hash256Concat(nKey[:], []byte{tag}, bbuf[:], []byte(addr))
		return int(binary.LittleEndian.Uint64(sum[:8]) % uint64(addrBucketSlotCap))
	}
	sum := hash256Concat(nKey[:], []byte{tag}, bbuf[:], key)
	return int(binary.LittleEndian.Uint64(sum[:8]) % uint64(addrBucketSlotCap))
}

func assignAddrBuckets(r *AddrRecord) {
	assignAddrBucketsWithKey(addrmanKey{}, r, "")
}

func assignAddrBucketsWithKey(nKey addrmanKey, r *AddrRecord, sourceHost string) {
	if r == nil || r.Addr == "" {
		return
	}
	r.TriedBucket = triedBucketFor(nKey, r.Addr)
	r.NewBucket = newBucketFor(nKey, r.Addr, r.Group16, sourceHost)
}

func (r *AddrRecord) newRefCount() int {
	if r == nil {
		return 0
	}
	return len(r.NewRefs)
}

func (r *AddrRecord) hasNewBucket(bucket int) bool {
	if r == nil {
		return false
	}
	for _, ref := range r.NewRefs {
		if ref.Bucket == bucket {
			return true
		}
	}
	return false
}

func (r *AddrRecord) syncPrimaryNewBucket() {
	if r == nil {
		return
	}
	if len(r.NewRefs) == 0 {
		r.NewBucket = 0
		return
	}
	r.NewBucket = r.NewRefs[0].Bucket
}

func (r *AddrRecord) clearNewRefs() {
	if r == nil {
		return
	}
	r.NewRefs = nil
	r.NewBucket = 0
}

func (b *AddrBook) clearNewRefsLocked(r *AddrRecord) {
	if r == nil {
		return
	}
	for _, ref := range r.NewRefs {
		b.clearNewOccLocked(ref.Bucket, ref.Slot, r.Addr)
	}
	r.clearNewRefs()
}

func (b *AddrBook) occupantNewLocked(bucket, slot int) *AddrRecord {
	if b.newSlotOcc == nil {
		return nil
	}
	addr, ok := b.newSlotOcc[addrSlotKey(bucket, slot)]
	if !ok || addr == "" {
		return nil
	}
	return b.by[addr]
}

func (b *AddrBook) occupantTriedLocked(bucket, slot int) *AddrRecord {
	if b.triedSlotOcc == nil {
		return nil
	}
	addr, ok := b.triedSlotOcc[addrSlotKey(bucket, slot)]
	if !ok || addr == "" {
		return nil
	}
	return b.by[addr]
}

func (b *AddrBook) setNewOccLocked(bucket, slot int, addr string) {
	if b.newSlotOcc == nil {
		b.newSlotOcc = make(map[uint32]string)
	}
	b.newSlotOcc[addrSlotKey(bucket, slot)] = addr
}

func (b *AddrBook) clearNewOccLocked(bucket, slot int, addr string) {
	if b.newSlotOcc == nil {
		return
	}
	k := addrSlotKey(bucket, slot)
	if b.newSlotOcc[k] == addr {
		delete(b.newSlotOcc, k)
	}
}

func (b *AddrBook) setTriedOccLocked(bucket, slot int, addr string) {
	if b.triedSlotOcc == nil {
		b.triedSlotOcc = make(map[uint32]string)
	}
	b.triedSlotOcc[addrSlotKey(bucket, slot)] = addr
}

func (b *AddrBook) clearTriedOccLocked(bucket, slot int, addr string) {
	if b.triedSlotOcc == nil {
		return
	}
	k := addrSlotKey(bucket, slot)
	if b.triedSlotOcc[k] == addr {
		delete(b.triedSlotOcc, k)
	}
}

func (b *AddrBook) clearAllOccForLocked(r *AddrRecord) {
	if r == nil {
		return
	}
	for _, ref := range r.NewRefs {
		b.clearNewOccLocked(ref.Bucket, ref.Slot, r.Addr)
	}
	if r.TriedSlot >= 0 {
		b.clearTriedOccLocked(r.TriedBucket, r.TriedSlot, r.Addr)
	}
}

func (b *AddrBook) rebuildOccLocked() {
	b.newSlotOcc = make(map[uint32]string)
	b.triedSlotOcc = make(map[uint32]string)
	for _, rec := range b.by {
		if rec == nil {
			continue
		}
		if rec.isTried() && rec.TriedSlot >= 0 {
			b.setTriedOccLocked(rec.TriedBucket, rec.TriedSlot, rec.Addr)
			continue
		}
		for _, ref := range rec.NewRefs {
			b.setNewOccLocked(ref.Bucket, ref.Slot, rec.Addr)
		}
	}
}

func (b *AddrBook) isTerribleLocked(rec *AddrRecord, now int64) bool {
	if rec == nil {
		return true
	}
	if now-rec.LastTry <= 60 {
		return false
	}
	if rec.LastSeen > now+addrMaxFutureOffsetSec {
		return true
	}
	if now-rec.LastSeen > addrStaleAfterSeconds {
		return true
	}
	if rec.Successes == 0 && rec.Attempts >= 3 {
		return true
	}
	if rec.Successes > 0 && rec.Attempts-rec.Successes >= 10 && now-rec.LastTry > 7*24*3600 {
		return true
	}
	return false
}

func (b *AddrBook) placeTriedSlotLocked(r *AddrRecord) {
	if b == nil || r == nil {
		return
	}
	b.clearNewRefsLocked(r)
	if r.TriedSlot >= 0 {
		b.clearTriedOccLocked(r.TriedBucket, r.TriedSlot, r.Addr)
	}
	bk := triedBucketFor(b.nKey, r.Addr)
	slot := bucketPosition(b.nKey, false, bk, r.Addr)
	if occ := b.occupantTriedLocked(bk, slot); occ != nil && occ.Addr != r.Addr {
		if occ.LastSeen <= r.LastSeen || b.isTerribleLocked(occ, r.LastSeen) {
			b.clearTriedOccLocked(occ.TriedBucket, occ.TriedSlot, occ.Addr)
			occ.TriedSlot = -1
			// Keep occ.Tried=true so a pure collision replacement doesn't turn into
			// an immediate “extra” demotion. Cap enforcement (enforceTriedCapLocked)
			// will handle reducing the tried population to the exact table size.
		} else {
			r.TriedBucket = bk
			r.TriedSlot = -1
			return
		}
	}
	r.TriedBucket = bk
	r.TriedSlot = slot
	b.setTriedOccLocked(bk, slot, r.Addr)
	b.trimTriedBucketLocked(bk, addrBucketSlotCap)
}

func (b *AddrBook) placeNewRefLocked(r *AddrRecord, sourceHost string, force bool) {
	if b == nil || r == nil || r.isTried() {
		return
	}
	nRefs := len(r.NewRefs)
	if nRefs >= addrNewBucketsPerAddress {
		return
	}
	if !force && nRefs > 0 {
		factor := 1 << nRefs
		h := addrBucketHash(r.Addr + "#ref#" + strconv.Itoa(nRefs) + b.nKey.hex()[:8])
		if int(h%uint64(factor)) != 0 {
			return
		}
	}
	for attempt := 0; attempt < addrNewBucketsPerAddress; attempt++ {
		bk := newBucketFor(b.nKey, r.Addr, r.Group16, sourceHost)
		if attempt > 0 {
			bk = int((uint64(bk) + uint64(attempt)*9973) % uint64(addrNewBucketCount))
		}
		if r.hasNewBucket(bk) {
			continue
		}
		slot := bucketPosition(b.nKey, true, bk, r.Addr)
		if occ := b.occupantNewLocked(bk, slot); occ != nil && occ.Addr != r.Addr {
			now := r.LastSeen
			if now == 0 {
				now = occ.LastSeen
			}
			canReplace := b.isTerribleLocked(occ, now) || (occ.newRefCount() > 1 && r.newRefCount() == 0)
			if !canReplace {
				continue
			}
			b.removeNewRefLocked(occ, bk, slot)
			if occ.newRefCount() == 0 && !occ.isTried() {
				b.removeAddrLocked(occ.Addr)
			}
		}
		r.NewRefs = append(r.NewRefs, AddrSlotRef{Bucket: bk, Slot: slot})
		r.syncPrimaryNewBucket()
		b.setNewOccLocked(bk, slot, r.Addr)
		b.trimNewBucketLocked(bk, addrBucketSlotCap)
		return
	}
}

func (b *AddrBook) removeNewRefLocked(r *AddrRecord, bucket, slot int) {
	if r == nil {
		return
	}
	out := r.NewRefs[:0]
	for _, ref := range r.NewRefs {
		if ref.Bucket == bucket && ref.Slot == slot {
			b.clearNewOccLocked(ref.Bucket, ref.Slot, r.Addr)
			continue
		}
		out = append(out, ref)
	}
	r.NewRefs = out
	r.syncPrimaryNewBucket()
}

func (b *AddrBook) recountNewBucketLocked(bucket int) int {
	n := 0
	for _, rec := range b.by {
		if rec == nil || rec.isTried() {
			continue
		}
		for _, ref := range rec.NewRefs {
			if ref.Bucket == bucket {
				n++
			}
		}
	}
	return n
}

func (b *AddrBook) recountTriedBucketLocked(bucket int) int {
	n := 0
	for _, rec := range b.by {
		if rec == nil || !rec.isTried() {
			continue
		}
		if rec.TriedBucket == bucket && rec.TriedSlot >= 0 {
			n++
		}
	}
	return n
}
