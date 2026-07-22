// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"fmt"
	"sync"
	"time"

	"dogego/wire"
)

const (
	DefaultMaxOrphans     = 100
	MaxOrphansCap         = 1000
	// defaultMaxOrphanBytes matches ~MAX_STANDARD_TX_WEIGHT/4 for legacy txs (Core orphan size cap).
	defaultMaxOrphanBytes = 100_000
	// maxOrphanTxWeight matches Core MAX_STANDARD_TX_WEIGHT orphan admission (net_processing.cpp).
	maxOrphanTxWeight = 400_000
)

// OrphanPool holds transactions whose prevouts are not yet available (P2P orphan handling).
type OrphanPool struct {
	mu        sync.Mutex
	max       int
	maxBytes  int
	byID      map[string][]byte
	byParent  map[string]map[string]struct{}
	byPeer    map[string]map[string]struct{} // peer addr -> orphan display txid
	expiresAt map[string]int64               // display txid -> unix expiry (Core COrphanTx nExpireTime)
}

// NewOrphanPool creates an orphan pool with Core-like default limits.
func NewOrphanPool(maxEntries int) *OrphanPool {
	if maxEntries <= 0 {
		maxEntries = DefaultMaxOrphans
	}
	if maxEntries > MaxOrphansCap {
		maxEntries = MaxOrphansCap
	}
	return &OrphanPool{
		max:       maxEntries,
		maxBytes:  defaultMaxOrphanBytes,
		byID:      make(map[string][]byte),
		byParent:  make(map[string]map[string]struct{}),
		byPeer:    make(map[string]map[string]struct{}),
		expiresAt: make(map[string]int64),
	}
}

// MaxEntries returns the configured orphan pool capacity.
func (o *OrphanPool) MaxEntries() int {
	if o == nil {
		return 0
	}
	return o.max
}

// Count returns the number of stored orphan transactions.
func (o *OrphanPool) Count() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.byID)
}

// Add stores raw tx bytes keyed by display txid; parentTxIDs lists missing mempool parents.
// fromPeer is the P2P peer that announced the tx (Core EraseOrphansFor on disconnect).
func (o *OrphanPool) Add(raw []byte, parentTxIDs []string, fromPeer string) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("orphan: empty tx")
	}
	if len(raw) > o.maxBytes {
		return "", fmt.Errorf("orphan: tx too large (%d bytes)", len(raw))
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return "", fmt.Errorf("orphan: decode: %w", err)
	}
	if wt, werr := legacyTxWeight(tx); werr == nil && wt >= maxOrphanTxWeight {
		return "", fmt.Errorf("orphan: tx too large (weight %d)", wt)
	}
	id := txidDisplayHex(tx.TxHash())
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, dup := o.byID[id]; dup {
		return id, nil
	}
	if len(o.byID) >= o.max && o.byID[id] == nil {
		o.evictOneLocked()
	}
	cp := append([]byte(nil), raw...)
	o.byID[id] = cp
	o.expiresAt[id] = time.Now().Unix() + OrphanTxExpireTime
	for _, p := range parentTxIDs {
		if p == "" {
			continue
		}
		if o.byParent[p] == nil {
			o.byParent[p] = make(map[string]struct{})
		}
		o.byParent[p][id] = struct{}{}
	}
	if fromPeer != "" {
		if o.byPeer[fromPeer] == nil {
			o.byPeer[fromPeer] = make(map[string]struct{})
		}
		o.byPeer[fromPeer][id] = struct{}{}
	}
	return id, nil
}

// RemoveByPeer drops all orphans announced by a disconnected peer (Core EraseOrphansFor).
func (o *OrphanPool) RemoveByPeer(peer string) int {
	if o == nil || peer == "" {
		return 0
	}
	o.mu.Lock()
	ids := o.byPeer[peer]
	if len(ids) == 0 {
		o.mu.Unlock()
		return 0
	}
	toDrop := make([]string, 0, len(ids))
	for id := range ids {
		toDrop = append(toDrop, id)
	}
	delete(o.byPeer, peer)
	o.mu.Unlock()
	for _, id := range toDrop {
		o.Remove(id)
	}
	return len(toDrop)
}

// Remove drops an orphan by display txid.
func (o *OrphanPool) Remove(displayTxid string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.removeLocked(displayTxid)
}

// evictOneLocked drops one orphan when the pool is at capacity (Core random eviction).
func (o *OrphanPool) evictOneLocked() {
	for id := range o.byID {
		o.removeLocked(id)
		return
	}
}

func (o *OrphanPool) removeLocked(displayTxid string) {
	raw, ok := o.byID[displayTxid]
	if !ok {
		return
	}
	delete(o.byID, displayTxid)
	delete(o.expiresAt, displayTxid)
	for peer, m := range o.byPeer {
		delete(m, displayTxid)
		if len(m) == 0 {
			delete(o.byPeer, peer)
		}
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return
	}
	for _, in := range tx.Vin {
		if isNullOutpoint(in) {
			continue
		}
		pid := txidDisplayHex(in.PrevHash)
		if m := o.byParent[pid]; m != nil {
			delete(m, displayTxid)
			if len(m) == 0 {
				delete(o.byParent, pid)
			}
		}
	}
}

// ChildrenOf returns orphan raw txs that list parentTxid as a missing parent (copy of payloads).
func (o *OrphanPool) ChildrenOf(parentTxid string) [][]byte {
	o.mu.Lock()
	defer o.mu.Unlock()
	ids := o.byParent[parentTxid]
	if len(ids) == 0 {
		return nil
	}
	out := make([][]byte, 0, len(ids))
	for id := range ids {
		if raw, ok := o.byID[id]; ok {
			out = append(out, append([]byte(nil), raw...))
		}
	}
	return out
}

