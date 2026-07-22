// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"fmt"
	"slices"

	"dogego/wire"
)

func isNullOutpoint(in wire.TxIn) bool {
	var z [32]byte
	return in.PrevHash == z && in.PrevIdx == 0xffffffff
}

// mempoolTxMap returns pooled txs keyed by RPC display txid (64 lowercase hex).
func (p *Pool) mempoolTxMap() (map[string]*wire.Tx, error) {
	entries, err := p.sortedTxEntries()
	if err != nil {
		return nil, err
	}
	out := make(map[string]*wire.Tx, len(entries))
	for _, e := range entries {
		tx, err := wire.DeserializeTx(e.raw)
		if err != nil {
			return nil, fmt.Errorf("mempool corrupt %s: %w", e.txid, err)
		}
		got := txidDisplayHex(tx.TxHash())
		if got != e.txid {
			return nil, fmt.Errorf("mempool txid mismatch for %s", e.txid)
		}
		out[e.txid] = tx
	}
	return out, nil
}

// SpendEdges builds parent/child links for txs whose inputs spend outputs of other pooled txs.
func (p *Pool) SpendEdges() (txs map[string]*wire.Tx, parents map[string][]string, children map[string][]string, err error) {
	return p.spendEdges()
}

// spendEdges builds parent/child links for txs whose inputs spend outputs of other pooled txs.
func (p *Pool) spendEdges() (txs map[string]*wire.Tx, parents map[string][]string, children map[string][]string, err error) {
	txs, err = p.mempoolTxMap()
	if err != nil {
		return nil, nil, nil, err
	}
	parents = make(map[string][]string)
	children = make(map[string][]string)
	for id := range txs {
		parents[id] = nil
		children[id] = nil
	}
	for txid, tx := range txs {
		seenP := make(map[string]bool)
		for _, in := range tx.Vin {
			if isNullOutpoint(in) {
				continue
			}
			pid := txidDisplayHex(in.PrevHash)
			if _, ok := txs[pid]; !ok {
				continue
			}
			if seenP[pid] {
				continue
			}
			seenP[pid] = true
			parents[txid] = append(parents[txid], pid)
			children[pid] = append(children[pid], txid)
		}
	}
	return txs, parents, children, nil
}

// MempoolAncestorTxIDs returns distinct in-mempool ancestors of seed (not including seed), sorted lexically.
func (p *Pool) MempoolAncestorTxIDs(seed string) ([]string, error) {
	raw, err := p.GetRawByTxID(seed)
	if err != nil {
		return nil, err
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return nil, fmt.Errorf("mempool corrupt: %w", err)
	}
	seedID := txidDisplayHex(tx.TxHash())
	_, parents, _, err := p.spendEdges()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var out []string
	stack := append([]string(nil), parents[seedID]...)
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[cur] {
			continue
		}
		seen[cur] = true
		out = append(out, cur)
		stack = append(stack, parents[cur]...)
	}
	slices.Sort(out)
	return out, nil
}

// admissionAncestorTxIDs returns in-mempool ancestors for a candidate tx (parents via inputs, not including the candidate).
func (p *Pool) admissionAncestorTxIDs(tx *wire.Tx) ([]string, error) {
	txs, parents, _, err := p.spendEdges()
	if err != nil {
		return nil, err
	}
	seenDirect := make(map[string]bool)
	var stack []string
	for _, in := range tx.Vin {
		if isNullOutpoint(in) {
			continue
		}
		pid := txidDisplayHex(in.PrevHash)
		if _, ok := txs[pid]; !ok {
			continue
		}
		if seenDirect[pid] {
			continue
		}
		seenDirect[pid] = true
		stack = append(stack, pid)
	}
	seen := make(map[string]bool)
	var out []string
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[cur] {
			continue
		}
		seen[cur] = true
		out = append(out, cur)
		stack = append(stack, parents[cur]...)
	}
	slices.Sort(out)
	return out, nil
}

// AdmissionAncestorCount returns in-mempool ancestor count for a candidate tx (excludes the candidate).
func (p *Pool) AdmissionAncestorCount(tx *wire.Tx) (int, error) {
	anc, err := p.admissionAncestorTxIDs(tx)
	if err != nil {
		return 0, err
	}
	return len(anc), nil
}

// MempoolDescendantTxIDs returns distinct in-mempool descendants of seed (not including seed), sorted lexically.
func (p *Pool) MempoolDescendantTxIDs(seed string) ([]string, error) {
	raw, err := p.GetRawByTxID(seed)
	if err != nil {
		return nil, err
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return nil, fmt.Errorf("mempool corrupt: %w", err)
	}
	seedID := txidDisplayHex(tx.TxHash())
	_, _, children, err := p.spendEdges()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var out []string
	queue := append([]string(nil), children[seedID]...)
	for head := 0; head < len(queue); head++ {
		cid := queue[head]
		if seen[cid] {
			continue
		}
		seen[cid] = true
		out = append(out, cid)
		queue = append(queue, children[cid]...)
	}
	slices.Sort(out)
	return out, nil
}
