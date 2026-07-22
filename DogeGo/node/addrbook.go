// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"dogego/chain"
	"dogego/wire"
)

const (
	addrTryCooldownBase   = 5 * time.Minute
	addrFeelerMinInterval = 2 * time.Minute
	// Core addrman capacity: 256×64 tried slots, 1024×64 new slots (src/addrman.h).
	maxAddrBookTried = addrTriedBucketCount * addrBucketSlotCap
	maxAddrBookNew   = addrNewBucketCount * addrBucketSlotCap
	maxAddrBookTotal = maxAddrBookTried + maxAddrBookNew
	addrStaleAfterSeconds = 30 * 24 * 3600
	// Core addrman: ignore CAddress nTime more than 10 minutes in the future.
	addrMaxFutureOffsetSec = 10 * 60
)

// AddrRecord tracks Core-style reachability metadata for one host:port.
type AddrRecord struct {
	Addr      string `json:"addr"`
	LastSeen  int64  `json:"last_seen,omitempty"`
	LastTry   int64  `json:"last_try,omitempty"`
	Attempts  int    `json:"attempts,omitempty"`
	Successes int    `json:"successes,omitempty"`
	// Tried is set after a successful outbound handshake (Core addrman "tried" table).
	Tried bool `json:"tried,omitempty"`
	// Services is the last known NODE_* bits from an addr gossip (Core CAddress nServices).
	Services uint64 `json:"services,omitempty"`
	// Group16 is the IPv4/IPv6 /16 group for new-table diversity (Core addrman source groups).
	Group16 string `json:"group16,omitempty"`
	// TriedBucket / NewBucket are Core addrman hash buckets (256 tried, 1024 new).
	TriedBucket int `json:"tried_bucket,omitempty"`
	TriedSlot   int `json:"tried_slot,omitempty"` // vvTried[bucket][slot]; -1 when unset
	NewBucket   int `json:"new_bucket,omitempty"` // primary = NewRefs[0]
	// NewRefs are vvNew multi-refs (Core ADDRMAN_NEW_BUCKETS_PER_ADDRESS = 8).
	NewRefs []AddrSlotRef `json:"new_refs,omitempty"`
}

// AddrBook is a Core-shaped addrman: tried/new tables with per-bucket slots and multi-ref new.
type AddrBook struct {
	mu           sync.Mutex
	by           map[string]*AddrRecord
	order        []string
	dirty        bool
	nKey         addrmanKey
	newSlotOcc   map[uint32]string // bucket<<6|slot → addr
	triedSlotOcc map[uint32]string
}

func NewAddrBook() *AddrBook {
	return &AddrBook{
		by:           make(map[string]*AddrRecord),
		nKey:         newAddrmanKey(),
		newSlotOcc:   make(map[uint32]string),
		triedSlotOcc: make(map[uint32]string),
	}
}

func addrSlotKey(bucket, slot int) uint32 {
	return uint32(bucket)<<6 | uint32(slot&63)
}

func (b *AddrBook) record(addr string) *AddrRecord {
	if r, ok := b.by[addr]; ok {
		return r
	}
	r := &AddrRecord{Addr: addr, LastSeen: time.Now().Unix(), TriedSlot: -1}
	b.by[addr] = r
	b.order = append(b.order, addr)
	b.evictIfOverCap()
	b.dirty = true
	return r
}

func (b *AddrBook) ensurePlacedLocked(r *AddrRecord, sourceHost string) {
	if r == nil {
		return
	}
	if r.isTried() {
		if r.TriedSlot < 0 {
			b.placeTriedSlotLocked(r)
		}
		return
	}
	if len(r.NewRefs) == 0 {
		b.assignBucketsLocked(r, sourceHost)
	}
}

func (r *AddrRecord) isTried() bool {
	return r != nil && r.Tried
}

func (b *AddrBook) triedCountLocked() int {
	n := 0
	for _, rec := range b.by {
		if rec != nil && rec.isTried() {
			n++
		}
	}
	return n
}

func (b *AddrBook) evictIfOverCap() {
	for len(b.order) > maxAddrBookTotal || b.newCountLocked() > maxAddrBookNew {
		dropIdx := -1
		for i, addr := range b.order {
			rec := b.by[addr]
			if rec != nil && !rec.isTried() {
				dropIdx = i
				break
			}
		}
		if dropIdx < 0 {
			dropIdx = 0
		}
		drop := b.order[dropIdx]
		b.order = append(b.order[:dropIdx], b.order[dropIdx+1:]...)
		delete(b.by, drop)
	}
}

// enforceTriedCapLocked demotes tried entries when over Core's tried table size (bucket-spread eviction).
func (b *AddrBook) enforceTriedCapLocked() {
	b.backfillBucketsLocked()
	for b.triedCountLocked() > maxAddrBookTried {
		before := b.triedCountLocked()
		b.demoteTriedForCapLocked()
		if b.triedCountLocked() >= before {
			break
		}
	}
}

func (b *AddrBook) newCountLocked() int {
	n := 0
	for _, rec := range b.by {
		if rec != nil && !rec.isTried() {
			n++
		}
	}
	return n
}

func (b *AddrBook) pruneStaleLocked(nowUnix int64) {
	if nowUnix <= 0 {
		nowUnix = time.Now().Unix()
	}
	cutoff := nowUnix - addrStaleAfterSeconds
	var drop []string
	for _, addr := range b.order {
		rec := b.by[addr]
		if rec == nil {
			continue
		}
		if rec.LastSeen > 0 && rec.LastSeen < cutoff {
			drop = append(drop, addr)
		}
	}
	for _, addr := range drop {
		for i, a := range b.order {
			if a == addr {
				b.order = append(b.order[:i], b.order[i+1:]...)
				break
			}
		}
		delete(b.by, addr)
	}
	if len(drop) > 0 {
		b.dirty = true
	}
}

// AddSeen learns or refreshes an address (inbound addr, feeler ok, gossip).
func (b *AddrBook) AddSeen(addr string) {
	b.AddSeenMeta(addr, 0, 0)
}

// AddSeenMeta learns an address with Core CAddress nTime / nServices metadata.
func (b *AddrBook) AddSeenMeta(addr string, services uint64, seenUnix int64) {
	b.AddSeenFrom(addr, services, seenUnix, "")
}

// AddSeenFrom learns a routable address; fromPeer is optional gossip source host:port (Core source group).
func (b *AddrBook) AddSeenFrom(addr string, services uint64, seenUnix int64, fromPeer string) {
	if b == nil || addr == "" || !IsHostPortRoutable(addr) {
		return
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now().Unix()
	b.pruneStaleLocked(now)
	r := b.record(addr)
	if services != 0 {
		r.Services = services
	}
	r.LastSeen = normalizeAddrSeenUnix(seenUnix, now)
	r.Group16 = addrGroup16(ip)
	srcHost := ""
	if fromPeer != "" {
		if h, _, err := net.SplitHostPort(fromPeer); err == nil {
			srcHost = h
			if src := net.ParseIP(h); src != nil {
				if g := addrGroup16(src); g != "" {
					r.Group16 = g
				}
			}
		}
	}
	if r.isTried() {
		b.ensurePlacedLocked(r, srcHost)
	} else if len(r.NewRefs) == 0 {
		b.assignBucketsLocked(r, srcHost)
		b.enforceNewBucketSlotCapLocked(r.NewBucket)
	} else {
		b.placeNewRefLocked(r, srcHost, false)
		for _, ref := range r.NewRefs {
			b.enforceNewBucketSlotCapLocked(ref.Bucket)
		}
	}
	b.dirty = true
}

// IsTried reports whether addr is in the tried table (successful handshake).
func (b *AddrBook) IsTried(addr string) bool {
	if b == nil || addr == "" {
		return false
	}
	b.mu.Lock()
	rec := b.by[addr]
	b.mu.Unlock()
	return rec != nil && rec.isTried()
}

// NoteTry records an outbound dial attempt.
func (b *AddrBook) NoteTry(addr string) {
	if b == nil || addr == "" || !IsHostPortRoutable(addr) {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	r := b.record(addr)
	b.ensurePlacedLocked(r, "")
	now := time.Now().Unix()
	r.LastTry = now
	r.Attempts++
	b.dirty = true
}

// NoteAddnodePersistent seeds a configured addnode into the tried table (Core addnode is tried-first on dial).
func (b *AddrBook) NoteAddnodePersistent(addr string) {
	if b == nil || addr == "" || !IsHostPortRoutable(addr) {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	r := b.record(addr)
	r.Tried = true
	r.LastSeen = time.Now().Unix()
	b.placeTriedSlotLocked(r)
	b.enforceTriedCapLocked()
	b.dirty = true
}

// RecordOutboundDialTry marks an outbound TCP attempt (Core addrman try).
func RecordOutboundDialTry(book *AddrBook, addr string) {
	if book != nil && addr != "" {
		book.NoteTry(addr)
	}
}

// RecordOutboundHandshakeResult records handshake outcome (success → tried table).
func RecordOutboundHandshakeResult(book *AddrBook, addr string, err error) {
	if book == nil || addr == "" {
		return
	}
	if err != nil {
		book.NoteFailure(addr)
		return
	}
	book.NoteSuccess(addr)
}

// RecordOutboundPeerHandshake records handshake outcome plus version services/start height.
func RecordOutboundPeerHandshake(book *AddrBook, scorer *BlockPeerScorer, addr string, dv *wire.DecodedVersion, err error) {
	RecordOutboundHandshakeResult(book, addr, err)
	if err != nil || dv == nil {
		return
	}
	if book != nil {
		book.NoteServices(addr, dv.Services)
	}
	if scorer != nil {
		scorer.NotePeerHandshake(addr, dv.Services, dv.StartHeight)
	}
}

// TouchSeen refreshes LastSeen when a peer delivers useful work (blocks) without changing tried state.
func (b *AddrBook) TouchSeen(addr string) {
	if b == nil || addr == "" || !IsHostPortRoutable(addr) {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if r := b.by[addr]; r != nil {
		r.LastSeen = time.Now().Unix()
		b.dirty = true
	}
}

// NoteRelayTry records an outbound DGR relay dial attempt (accepts DNS host:port).
func (b *AddrBook) NoteRelayTry(addr string) {
	if b == nil || !IsRelayHostPort(addr) {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	r := b.record(addr)
	now := time.Now().Unix()
	r.LastTry = now
	r.Attempts++
	b.dirty = true
}

// NoteRelaySuccess records a successful DGR relay REGISTER on addr.
func (b *AddrBook) NoteRelaySuccess(addr string) {
	if b == nil || !IsRelayHostPort(addr) {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	r := b.record(addr)
	r.Successes++
	r.Tried = true
	r.LastSeen = time.Now().Unix()
	b.placeTriedSlotLocked(r)
	b.enforceTriedBucketSlotCapLocked(r.TriedBucket)
	b.enforceTriedCapLocked()
	b.dirty = true
}

// NoteRelayFailure records a failed DGR relay dial or REGISTER on addr.
func (b *AddrBook) NoteRelayFailure(addr string) {
	if b == nil || !IsRelayHostPort(addr) {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	r := b.record(addr)
	now := time.Now().Unix()
	if r.LastTry == 0 {
		r.LastTry = now
	}
	r.Attempts++
	b.dirty = true
}

// NoteSuccess records a completed handshake on addr.
func (b *AddrBook) NoteSuccess(addr string) {
	if b == nil || addr == "" || !IsHostPortRoutable(addr) {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	r := b.record(addr)
	r.Successes++
	r.Tried = true
	r.LastSeen = time.Now().Unix()
	b.placeTriedSlotLocked(r)
	b.enforceTriedBucketSlotCapLocked(r.TriedBucket)
	b.enforceTriedCapLocked()
	b.dirty = true
}

// NoteServices updates last-known NODE_* bits from version or addr gossip.
func (b *AddrBook) NoteServices(addr string, services uint64) {
	if b == nil || addr == "" || !IsHostPortRoutable(addr) || services == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	r := b.record(addr)
	r.Services = services
	b.dirty = true
}

// NoteFailure records a failed dial or handshake (extends cooldown via attempts > successes).
func (b *AddrBook) NoteFailure(addr string) {
	if b == nil || addr == "" || !IsHostPortRoutable(addr) {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	r := b.record(addr)
	now := time.Now().Unix()
	if r.LastTry == 0 {
		r.LastTry = now
	}
	r.Attempts++
	b.dirty = true
}

// RelayDialScore returns Core addrman dial score for a DGR relay QUIC host:port.
func (b *AddrBook) RelayDialScore(addr string) int {
	if b == nil || addr == "" {
		return -1 << 30
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	rec := b.by[addr]
	if rec == nil {
		return 0
	}
	return b.dialScoreLocked(rec, time.Now())
}

func (b *AddrBook) groupHistogramLocked() map[string]int {
	hist := make(map[string]int)
	for _, rec := range b.by {
		if rec != nil && rec.Group16 != "" {
			hist[rec.Group16]++
		}
	}
	return hist
}

func (b *AddrBook) dialScoreLocked(rec *AddrRecord, now time.Time) int {
	return b.dialScoreWithGroupsLocked(rec, now, nil)
}

func (b *AddrBook) dialScoreWithGroupsLocked(rec *AddrRecord, now time.Time, hist map[string]int) int {
	if rec == nil {
		return -1 << 30
	}
	score := rec.dialScore(now)
	if rec.Group16 != "" {
		n := 1
		if hist != nil {
			if v, ok := hist[rec.Group16]; ok {
				n = v
			}
		}
		score -= (n - 1) * 8
	}
	return score
}

// groupsForPeersLocked maps connected host:ports to /16 groups (for outbound dial spread).
func (b *AddrBook) groupsForPeersLocked(peers map[string]struct{}) map[string]struct{} {
	if len(peers) == 0 {
		return nil
	}
	used := make(map[string]struct{})
	for a := range peers {
		if rec := b.by[a]; rec != nil && rec.Group16 != "" {
			used[rec.Group16] = struct{}{}
			continue
		}
		host, _, err := net.SplitHostPort(a)
		if err != nil {
			continue
		}
		if ip := net.ParseIP(host); ip != nil {
			if g := addrGroup16(ip); g != "" {
				used[g] = struct{}{}
			}
		}
	}
	return used
}

func (r *AddrRecord) dialScore(now time.Time) int {
	if r == nil {
		return -1 << 30
	}
	if r.LastTry > 0 && r.Attempts > r.Successes {
		cd := addrTryCooldownBase
		if r.Attempts > 3 {
			cd *= 2
		}
		if now.Before(time.Unix(r.LastTry, 0).Add(cd)) {
			return -1 << 30
		}
	}
	score := r.Successes*100 - (r.Attempts-r.Successes)*15
	if r.LastTry == 0 {
		score += 40
	}
	if r.Services&chain.NodeNetwork != 0 {
		score += 25
	}
	return score
}

func (r *AddrRecord) feelerOK(now time.Time) bool {
	if r == nil {
		return false
	}
	if r.LastTry == 0 {
		return true
	}
	if r.Attempts > r.Successes {
		return now.After(time.Unix(r.LastTry, 0).Add(addrTryCooldownBase))
	}
	return now.After(time.Unix(r.LastTry, 0).Add(addrFeelerMinInterval))
}

// PickBest chooses a relay dial target (addrbook score, then block peer scorer tie-break).
// prefer lists addnode host:ports tried before other learned addresses (Core addnode priority).
// Dial candidates prefer the tried table (successful handshake) before new gossip addresses.
func (b *AddrBook) PickBest(skip map[string]struct{}, primary string, scorer *BlockPeerScorer, prefer []string) string {
	if b == nil {
		return ""
	}
	b.mu.Lock()
	now := time.Now()
	for _, addr := range prefer {
		if addr == "" || addr == primary {
			continue
		}
		if _, busy := skip[addr]; busy {
			continue
		}
		rec := b.by[addr]
		if rec == nil {
			rec = &AddrRecord{Addr: addr, LastSeen: now.Unix()}
		}
		if b.dialScoreLocked(rec, now) >= -1<<29 {
			b.mu.Unlock()
			return addr
		}
	}
	connectedGroups := b.groupsForPeersLocked(skip)
	top := b.pickBestLockedDiverse(skip, primary, now, true, connectedGroups)
	if len(top) == 0 {
		top = b.pickBestLockedDiverse(skip, primary, now, false, connectedGroups)
	}
	b.mu.Unlock()
	if len(top) == 0 {
		return ""
	}
	if scorer != nil && len(top) > 1 {
		top = scorer.DialableOrder(top, primary)
	}
	return top[0]
}

func (b *AddrBook) pickBestLocked(skip map[string]struct{}, primary string, now time.Time, wantTried bool) []string {
	return b.pickBestLockedFiltered(skip, primary, now, wantTried, nil)
}

// pickBestLockedFiltered optionally skips addresses in avoidGroups (/16 spread for getaddr samples).
func (b *AddrBook) pickBestLockedFiltered(skip map[string]struct{}, primary string, now time.Time, wantTried bool, avoidGroups map[string]struct{}) []string {
	var top []string
	topScore := -1 << 30
	hist := b.groupHistogramLocked()
	for addr, rec := range b.by {
		if addr == "" || addr == primary {
			continue
		}
		if _, busy := skip[addr]; busy {
			continue
		}
		if rec.isTried() != wantTried {
			continue
		}
		if !IsHostPortRoutable(addr) {
			continue
		}
		if len(avoidGroups) > 0 && rec.Group16 != "" {
			if _, dup := avoidGroups[rec.Group16]; dup {
				continue
			}
		}
		s := b.dialScoreWithGroupsLocked(rec, now, hist)
		if s < -1<<29 {
			continue
		}
		if len(top) == 0 || s > topScore {
			top = []string{addr}
			topScore = s
		} else if s == topScore {
			top = append(top, addr)
		}
	}
	return top
}

func (b *AddrBook) pickBestLockedDiverse(skip map[string]struct{}, primary string, now time.Time, wantTried bool, usedGroups map[string]struct{}) []string {
	if len(usedGroups) == 0 {
		return b.pickBestLocked(skip, primary, now, wantTried)
	}
	top := b.pickBestLockedFiltered(skip, primary, now, wantTried, usedGroups)
	if len(top) > 0 {
		return top
	}
	return b.pickBestLocked(skip, primary, now, wantTried)
}

func (b *AddrBook) noteSampleGroupLocked(addr string, usedGroups map[string]struct{}) {
	if usedGroups == nil {
		return
	}
	rec := b.by[addr]
	if rec != nil && rec.Group16 != "" {
		usedGroups[rec.Group16] = struct{}{}
	}
}

// PickFeeler returns an address for a short reachability probe.
func (b *AddrBook) PickFeeler(skip map[string]struct{}, primary string) string {
	if b == nil {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	var pick string
	var oldestTry int64 = -1
	for i := len(b.order) - 1; i >= 0; i-- {
		addr := b.order[i]
		if addr == "" || addr == primary {
			continue
		}
		if _, busy := skip[addr]; busy {
			continue
		}
		rec := b.by[addr]
		if rec == nil || rec.isTried() || !rec.feelerOK(now) {
			continue
		}
		if rec.LastTry == 0 {
			return addr
		}
		if oldestTry < 0 || rec.LastTry < oldestTry {
			oldestTry = rec.LastTry
			pick = addr
		}
	}
	return pick
}

// AddrBookStats returns tried vs new address counts (Core addrman analogue).
func (b *AddrBook) AddrBookStats() (tried, newAddrs int) {
	if b == nil {
		return 0, 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, rec := range b.by {
		if rec == nil {
			continue
		}
		if rec.isTried() {
			tried++
		} else {
			newAddrs++
		}
	}
	return tried, newAddrs
}

// AddrBookBucketStats reports Core-style hash bucket spread (256 tried / 1024 new buckets).
func (b *AddrBook) AddrBookBucketStats() (triedBucketsUsed, newBucketsUsed, triedMaxFill, newMaxFill int) {
	if b == nil {
		return 0, 0, 0, 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.backfillBucketsLocked()
	triedCounts := make([]int, addrTriedBucketCount)
	newCounts := make([]int, addrNewBucketCount)
	for _, rec := range b.by {
		if rec == nil {
			continue
		}
		if rec.isTried() {
			if rec.TriedBucket >= 0 && rec.TriedBucket < addrTriedBucketCount && rec.TriedSlot >= 0 {
				triedCounts[rec.TriedBucket]++
			}
			continue
		}
		if len(rec.NewRefs) == 0 {
			continue
		}
		for _, ref := range rec.NewRefs {
			if ref.Bucket >= 0 && ref.Bucket < addrNewBucketCount {
				newCounts[ref.Bucket]++
			}
		}
	}
	for _, n := range triedCounts {
		if n > 0 {
			triedBucketsUsed++
		}
		if n > triedMaxFill {
			triedMaxFill = n
		}
	}
	for _, n := range newCounts {
		if n > 0 {
			newBucketsUsed++
		}
		if n > newMaxFill {
			newMaxFill = n
		}
	}
	return triedBucketsUsed, newBucketsUsed, triedMaxFill, newMaxFill
}

// AddrSample returns up to n addresses for outbound getaddr replies (mostly tried, some new).
func (b *AddrBook) AddrSample(n int, defaultServices uint64) []wire.NetAddress {
	if b == nil || n <= 0 {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.pruneStaleLocked(now.Unix())
	skip := make(map[string]struct{})
	usedGroups := make(map[string]struct{})
	out := make([]wire.NetAddress, 0, n)
	triedQuota := n * 2 / 3
	if triedQuota < 1 {
		triedQuota = 1
	}
	for len(out) < triedQuota {
		top := b.pickBestLockedDiverse(skip, "", now, true, usedGroups)
		if len(top) == 0 {
			break
		}
		addr := top[0]
		skip[addr] = struct{}{}
		b.noteSampleGroupLocked(addr, usedGroups)
		if na, ok := b.netAddressLocked(addr, defaultServices, now.Unix()); ok && IsIPPortRoutable(na.IP, na.Port) {
			out = append(out, na)
		}
	}
	for len(out) < n {
		top := b.pickBestLockedDiverse(skip, "", now, false, usedGroups)
		if len(top) == 0 {
			break
		}
		addr := top[0]
		skip[addr] = struct{}{}
		b.noteSampleGroupLocked(addr, usedGroups)
		if na, ok := b.netAddressLocked(addr, defaultServices, now.Unix()); ok && IsIPPortRoutable(na.IP, na.Port) {
			out = append(out, na)
		}
	}
	return out
}

func (b *AddrBook) netAddressLocked(addr string, defaultServices uint64, nowUnix int64) (wire.NetAddress, bool) {
	rec := b.by[addr]
	if rec == nil {
		return wire.NetAddress{}, false
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return wire.NetAddress{}, false
	}
	var port uint16
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		return wire.NetAddress{}, false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return wire.NetAddress{}, false
	}
	svc := rec.Services
	if svc == 0 {
		svc = defaultServices
	}
	ts := uint32(rec.LastSeen)
	if ts == 0 {
		ts = uint32(nowUnix)
	}
	return wire.NetAddress{Time: ts, Services: svc, IP: ip, Port: port}, true
}

func addrNetworkName(ip net.IP, host string) string {
	if ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			return "ipv4"
		}
		if ip.To16() != nil {
			return "ipv6"
		}
	}
	if strings.HasSuffix(strings.ToLower(host), ".onion") {
		return "onion"
	}
	return ""
}

// NodeAddressRows returns Core getnodeaddresses-shaped rows (best peers first).
func (b *AddrBook) NodeAddressRows(count int, networkFilter string, defaultServices uint64) []map[string]interface{} {
	if b == nil {
		return nil
	}
	filter := strings.ToLower(strings.TrimSpace(networkFilter))
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.pruneStaleLocked(now.Unix())
	skip := make(map[string]struct{})
	limit := count
	if limit <= 0 {
		limit = len(b.order)
	}
	out := make([]map[string]interface{}, 0, limit)
	usedGroups := make(map[string]struct{})
	for len(out) < limit {
		top := b.pickBestLockedDiverse(skip, "", now, true, usedGroups)
		if len(top) == 0 {
			top = b.pickBestLockedDiverse(skip, "", now, false, usedGroups)
		}
		if len(top) == 0 {
			break
		}
		addr := top[0]
		skip[addr] = struct{}{}
		b.noteSampleGroupLocked(addr, usedGroups)
		na, ok := b.netAddressLocked(addr, defaultServices, now.Unix())
		if !ok || !IsIPPortRoutable(na.IP, na.Port) {
			continue
		}
		netName := addrNetworkName(na.IP, na.IP.String())
		if filter != "" && netName != filter {
			continue
		}
		row := map[string]interface{}{
			"time":     int64(na.Time),
			"services": na.Services,
			"address":  na.IP.String(),
			"port":     int(na.Port),
		}
		if netName != "" {
			row["network"] = netName
		}
		out = append(out, row)
	}
	return out
}

// Snapshot returns all known host:ports.
func (b *AddrBook) Snapshot() []string {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.order))
	copy(out, b.order)
	return out
}

// MergeFrom adds addresses without clearing existing metadata.
func (b *AddrBook) MergeFrom(addrs []string) {
	if b == nil {
		return
	}
	for _, a := range addrs {
		b.AddSeen(a)
	}
}

func (b *AddrBook) nKeyHex() string {
	if b == nil {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.nKey.hex()
}

// HasAddrmanKey reports whether a persisted Core-style nKey is loaded.
func (b *AddrBook) HasAddrmanKey() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.nKey != (addrmanKey{})
}

func (b *AddrBook) takeDirty() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.dirty {
		return false
	}
	b.dirty = false
	return true
}

func (b *AddrBook) markDirty() {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.dirty = true
	b.mu.Unlock()
}

// cloneForSave returns records in stable order for JSON persistence.
func (b *AddrBook) cloneForSave() []AddrRecord {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]AddrRecord, 0, len(b.order))
	for _, addr := range b.order {
		if r, ok := b.by[addr]; ok && r != nil {
			cp := *r
			if len(r.NewRefs) > 0 {
				cp.NewRefs = append([]AddrSlotRef(nil), r.NewRefs...)
			}
			out = append(out, cp)
		}
	}
	return out
}

// loadRecords replaces book contents from disk.
func (b *AddrBook) loadRecords(recs []AddrRecord) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now().Unix()
	b.by = make(map[string]*AddrRecord, len(recs))
	b.order = b.order[:0]
	b.newSlotOcc = make(map[uint32]string)
	b.triedSlotOcc = make(map[uint32]string)
	for _, r := range recs {
		if r.Addr == "" || !IsHostPortRoutable(r.Addr) {
			continue
		}
		cp := r
		cp.LastSeen = normalizeAddrSeenUnix(cp.LastSeen, now)
		if cp.TriedSlot == 0 && !cp.Tried {
			cp.TriedSlot = -1
		}
		if len(cp.NewRefs) > 0 {
			cp.NewRefs = append([]AddrSlotRef(nil), cp.NewRefs...)
		}
		b.by[r.Addr] = &cp
		b.order = append(b.order, r.Addr)
	}
	b.backfillBucketsLocked()
	b.rebuildOccLocked()
	b.pruneStaleLocked(now)
	b.enforceAllBucketSlotCapsLocked()
	b.enforceTriedCapLocked()
	b.dirty = false
}
