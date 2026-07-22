// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"fmt"
	"math"

	"dogego/wire"
)

// FeeRateKoinuPerKB maps RPC display txid to fee rate in koinu per kilobyte.
type FeeRateKoinuPerKB map[string]int64

// RemoveByTxID drops a pooled transaction by RPC display txid.
func (p *Pool) RemoveByTxID(displayTxid string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.removeByDisplayTxidLocked(displayTxid)
}

func deserializeTxCached(raw []byte) (*wire.Tx, error) {
	return wire.DeserializeTx(raw)
}

// mempoolLeafTxIDs returns txids that have no in-pool children (safe eviction leaves).
func (p *Pool) mempoolLeafTxIDs() ([]string, error) {
	_, _, children, err := p.spendEdges()
	if err != nil {
		return nil, err
	}
	var leaves []string
	for id, ch := range children {
		if len(ch) == 0 {
			leaves = append(leaves, id)
		}
	}
	return leaves, nil
}

// EvictLowestFeeRateLeaf removes one leaf transaction with the lowest known per-tx fee rate.
func (p *Pool) EvictLowestFeeRateLeaf(feeRates FeeRateKoinuPerKB) (string, error) {
	leaves, err := p.mempoolLeafTxIDs()
	if err != nil {
		return "", err
	}
	if len(leaves) == 0 {
		return "", nil
	}
	worst := leaves[0]
	worstRate := int64(math.MaxInt64)
	for _, id := range leaves {
		r := feeRates[id]
		if r < worstRate {
			worstRate = r
			worst = id
		}
	}
	if p.RemoveByTxID(worst) {
		return worst, nil
	}
	return "", nil
}

// EvictLowestAncestorFeeRateLeaf removes a leaf whose ancestor package has the lowest fee rate (koinu/kB).
func (p *Pool) EvictLowestAncestorFeeRateLeaf(feesKoinu map[string]int64, sizes map[string]int) (string, error) {
	leaves, err := p.mempoolLeafTxIDs()
	if err != nil {
		return "", err
	}
	if len(leaves) == 0 {
		return "", nil
	}
	worst := leaves[0]
	worstRate := int64(math.MaxInt64)
	for _, id := range leaves {
		pkg, err := p.PackageStatsForTxID(id, feesKoinu, sizes)
		if err != nil || pkg.AncestorSize <= 0 {
			continue
		}
		ancFees := p.MiningAncestorFeesKoinu(pkg, id)
		rate := ancFees * 1000 / int64(pkg.AncestorSize)
		if rate < worstRate {
			worstRate = rate
			worst = id
		}
	}
	if p.RemoveByTxID(worst) {
		return worst, nil
	}
	return "", nil
}

// AddWithEviction stores rawTx, evicting lowest ancestor-package-fee leaves until there is room.
// The candidate must pay a higher ancestor-package feerate than the evicted leaf (Core mempool replacement policy).
func (p *Pool) AddWithEviction(rawTx []byte, feesKoinu map[string]int64, sizes map[string]int) error {
	if len(rawTx) == 0 {
		return fmt.Errorf("empty tx")
	}
	tx, err := wire.DeserializeTx(rawTx)
	if err != nil {
		return err
	}
	candID := txidDisplayHex(tx.TxHash())
	candPkgFee := feesKoinu[candID]
	candSize := sizes[candID]
	if candSize <= 0 {
		candSize = len(rawTx)
	}
	const maxEvictions = 64
	for attempt := 0; attempt <= maxEvictions; attempt++ {
		p.PruneExpired()
		err := p.Add(rawTx)
		if err == nil {
			return nil
		}
		if !p.IsFull() {
			return err
		}
		evicted, eerr := p.EvictLowestAncestorFeeRateLeaf(feesKoinu, sizes)
		if eerr != nil {
			return err
		}
		if evicted == "" {
			return err
		}
		if evPkg, err := p.PackageStatsForTxID(evicted, feesKoinu, sizes); err == nil && evPkg.AncestorSize > 0 {
			p.trackPackageRemoved(evPkg.AncestorFeesKoinu, evPkg.AncestorSize)
		}
		if candSize > 0 && candPkgFee > 0 {
			evPkg, err := p.PackageStatsForTxID(evicted, feesKoinu, sizes)
			if err == nil && evPkg.AncestorSize > 0 {
				evAnc := p.MiningAncestorFeesKoinu(evPkg, evicted)
				evRate := evAnc * 1000 / int64(evPkg.AncestorSize)
				candRate := candPkgFee * 1000 / int64(candSize)
				if candRate <= evRate {
					return fmt.Errorf("mempool full: feerate %d <= evicted %d", candRate, evRate)
				}
			}
		}
	}
	return fmt.Errorf("mempool full (%d): eviction limit reached", p.max)
}
