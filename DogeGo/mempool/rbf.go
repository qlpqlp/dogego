// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import "dogego/wire"

// SpenderOfOutpoint returns the display txid of a pooled tx spending prevTxid:vout, or "" if none.
func (p *Pool) SpenderOfOutpoint(rpcPrevTxid string, vout uint32) string {
	p.mu.Lock()
	blobs := make([][]byte, 0, len(p.raw))
	for _, v := range p.raw {
		blobs = append(blobs, v)
	}
	p.mu.Unlock()
	for _, raw := range blobs {
		tx, err := wire.DeserializeTx(raw)
		if err != nil {
			continue
		}
		for _, in := range tx.Vin {
			if isNullOutpoint(in) {
				continue
			}
			if in.PrevIdx != vout {
				continue
			}
			if txidDisplayHex(in.PrevHash) == rpcPrevTxid {
				return txidDisplayHex(tx.TxHash())
			}
		}
	}
	return ""
}

// MempoolDescendantCount returns the number of in-mempool descendants (excluding the seed tx).
func (p *Pool) MempoolDescendantCount(displayTxid string) (int, error) {
	desc, err := p.MempoolDescendantTxIDs(displayTxid)
	if err != nil {
		return 0, err
	}
	return len(desc), nil
}

// MempoolAncestorCount returns the number of in-mempool ancestors (excluding the seed tx).
func (p *Pool) MempoolAncestorCount(displayTxid string) (int, error) {
	anc, err := p.MempoolAncestorTxIDs(displayTxid)
	if err != nil {
		return 0, err
	}
	return len(anc), nil
}

// ConflictPackageFeeSize returns descendant-package fee and size for a pooled conflict
// (BIP125 PaysForRBF / RemovesUnconfirmedSpends: the eviction set is the conflict plus
// its in-mempool descendants, not the ancestor package).
func (p *Pool) ConflictPackageFeeSize(displayTxid string, feesKoinu map[string]int64, sizes map[string]int) (fee int64, size int, ok bool) {
	st, err := p.PackageStatsForTxID(displayTxid, feesKoinu, sizes)
	if err != nil || st.DescendantSize <= 0 {
		return 0, 0, false
	}
	return st.DescendantFeesKoinu, st.DescendantSize, true
}

// RemoveCluster removes root and all in-mempool descendants; returns removed display txids.
func (p *Pool) RemoveCluster(rootDisplayTxid string) ([]string, error) {
	desc, err := p.MempoolDescendantTxIDs(rootDisplayTxid)
	if err != nil {
		return nil, err
	}
	removed := make([]string, 0, len(desc)+1)
	for i := len(desc) - 1; i >= 0; i-- {
		if p.RemoveByTxID(desc[i]) {
			removed = append(removed, desc[i])
		}
	}
	if p.RemoveByTxID(rootDisplayTxid) {
		removed = append(removed, rootDisplayTxid)
	}
	return removed, nil
}
