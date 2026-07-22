// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"fmt"
	"strings"

	"dogego/wire"
)

// feeDeltaPropagation records how a prioritised tx's fee_delta was applied to ancestor/descendant mining scores.
type feeDeltaPropagation struct {
	ancestors    map[string]int64 // descendantFeeBoost targets
	descendants  map[string]int64 // ancestorFeeBoost targets
}

// AddFeeDelta records a virtual fee delta for a pooled tx (Core prioritisetransaction).
// feeDeltaKoinu is in koinu (same units as RPC fee_delta); cumulative per txid.
// Propagates to ancestors' descendant-fee state and descendants' ancestor-fee state (Core CTxMemPool::PrioritiseTransaction).
func (p *Pool) AddFeeDelta(rpcTxid string, feeDeltaKoinu int64) error {
	if p == nil {
		return fmt.Errorf("mempool: nil pool")
	}
	rpcTxid = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(rpcTxid), "0x"))
	if len(rpcTxid) != 64 {
		return fmt.Errorf("txid must be 64 hex characters")
	}
	p.mu.Lock()
	if feeDeltaKoinu == 0 {
		if p.feeDelta != nil {
			delete(p.feeDelta, rpcTxid)
		}
		p.clearFeeDeltaLocked(rpcTxid)
		p.bumpChangeSeqLocked()
		p.mu.Unlock()
		return nil
	}
	if p.feeDelta == nil {
		p.feeDelta = make(map[string]int64)
	}
	p.feeDelta[rpcTxid] += feeDeltaKoinu
	if !p.containsTxIDLocked(rpcTxid) {
		p.bumpChangeSeqLocked()
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()
	anc, err := p.MempoolAncestorTxIDs(rpcTxid)
	if err != nil {
		return err
	}
	desc, err := p.MempoolDescendantTxIDs(rpcTxid)
	if err != nil {
		return err
	}
	p.mu.Lock()
	if _, ok := p.feeDeltaProp[rpcTxid]; !ok {
		p.propagateFeeDeltaLocked(rpcTxid, p.feeDelta[rpcTxid], anc, desc)
	} else {
		p.propagateFeeDeltaLocked(rpcTxid, feeDeltaKoinu, anc, desc)
	}
	p.bumpChangeSeqLocked()
	p.mu.Unlock()
	return nil
}

func (p *Pool) containsTxIDLocked(rpcTxid string) bool {
	for _, raw := range p.raw {
		tx, err := wire.DeserializeTx(raw)
		if err != nil {
			continue
		}
		if txidDisplayHex(tx.TxHash()) == rpcTxid {
			return true
		}
	}
	return false
}

// ExportFeeDeltas returns a copy of stored virtual fee deltas (Core mapDeltas), including latent entries for txs not in mempool.
func (p *Pool) ExportFeeDeltas() map[string]int64 {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.feeDelta) == 0 {
		return nil
	}
	out := make(map[string]int64, len(p.feeDelta))
	for id, d := range p.feeDelta {
		if d != 0 {
			out[id] = d
		}
	}
	return out
}

// RestoreFeeDeltas loads mapDeltas from a persisted mempool dump (after transactions are re-admitted).
func (p *Pool) RestoreFeeDeltas(deltas map[string]int64) {
	if p == nil || len(deltas) == 0 {
		return
	}
	for id, d := range deltas {
		if d == 0 {
			continue
		}
		p.mu.Lock()
		if p.feeDelta == nil {
			p.feeDelta = make(map[string]int64)
		}
		p.feeDelta[id] = d
		p.mu.Unlock()
		p.applyPendingFeeDeltaAfterAdd(id)
	}
}

func (p *Pool) applyPendingFeeDeltaAfterAdd(rpcTxid string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	delta := p.feeDelta[rpcTxid]
	_, propagated := p.feeDeltaProp[rpcTxid]
	inPool := p.containsTxIDLocked(rpcTxid)
	p.mu.Unlock()
	if delta == 0 || propagated || !inPool {
		return
	}
	anc, err := p.MempoolAncestorTxIDs(rpcTxid)
	if err != nil {
		return
	}
	desc, err := p.MempoolDescendantTxIDs(rpcTxid)
	if err != nil {
		return
	}
	p.mu.Lock()
	if p.feeDelta[rpcTxid] != 0 {
		if _, ok := p.feeDeltaProp[rpcTxid]; !ok {
			p.propagateFeeDeltaLocked(rpcTxid, p.feeDelta[rpcTxid], anc, desc)
		}
	}
	p.bumpChangeSeqLocked()
	p.mu.Unlock()
}

func (p *Pool) propagateFeeDeltaLocked(source string, delta int64, ancestors, descendants []string) {
	if delta == 0 {
		return
	}
	if p.descendantFeeBoost == nil {
		p.descendantFeeBoost = make(map[string]int64)
	}
	if p.ancestorFeeBoost == nil {
		p.ancestorFeeBoost = make(map[string]int64)
	}
	if p.feeDeltaProp == nil {
		p.feeDeltaProp = make(map[string]feeDeltaPropagation)
	}
	prop := p.feeDeltaProp[source]
	if prop.ancestors == nil {
		prop.ancestors = make(map[string]int64)
	}
	if prop.descendants == nil {
		prop.descendants = make(map[string]int64)
	}
	for _, id := range ancestors {
		p.descendantFeeBoost[id] += delta
		prop.ancestors[id] += delta
	}
	for _, id := range descendants {
		p.ancestorFeeBoost[id] += delta
		prop.descendants[id] += delta
	}
	p.feeDeltaProp[source] = prop
}

func (p *Pool) clearFeeDeltaLocked(rpcTxid string) {
	if p.feeDelta != nil {
		delete(p.feeDelta, rpcTxid)
	}
	prop, ok := p.feeDeltaProp[rpcTxid]
	if !ok {
		return
	}
	for id, v := range prop.ancestors {
		if p.descendantFeeBoost != nil {
			p.descendantFeeBoost[id] -= v
			if p.descendantFeeBoost[id] == 0 {
				delete(p.descendantFeeBoost, id)
			}
		}
	}
	for id, v := range prop.descendants {
		if p.ancestorFeeBoost != nil {
			p.ancestorFeeBoost[id] -= v
			if p.ancestorFeeBoost[id] == 0 {
				delete(p.ancestorFeeBoost, id)
			}
		}
	}
	delete(p.feeDeltaProp, rpcTxid)
}

func (p *Pool) clearAllFeeDeltasLocked() {
	p.feeDelta = nil
	p.descendantFeeBoost = nil
	p.ancestorFeeBoost = nil
	p.feeDeltaProp = nil
}

// FeeDeltaKoinu returns the cumulative virtual fee delta for a pooled tx (0 if none).
func (p *Pool) FeeDeltaKoinu(rpcTxid string) int64 {
	if p == nil {
		return 0
	}
	rpcTxid = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(rpcTxid), "0x"))
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.feeDelta == nil {
		return 0
	}
	return p.feeDelta[rpcTxid]
}

// MiningAncestorFeesKoinu returns ancestor-package fees for mining/eviction ordering (Core modified ancestor score).
func (p *Pool) MiningAncestorFeesKoinu(st PackageStats, txid string) int64 {
	if p == nil {
		return st.AncestorFeesKoinu
	}
	p.mu.Lock()
	boost := int64(0)
	if p.descendantFeeBoost != nil {
		boost = p.descendantFeeBoost[txid]
	}
	p.mu.Unlock()
	return st.AncestorFeesKoinu + boost
}

// ApplyFeeDeltas adds stored virtual fee deltas to a fee map (used for mining / eviction ordering).
func (p *Pool) ApplyFeeDeltas(fees map[string]int64) {
	if p == nil || fees == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, d := range p.feeDelta {
		if d != 0 {
			fees[id] += d
		}
	}
}
