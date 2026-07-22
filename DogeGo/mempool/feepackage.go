// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"fmt"

	"dogego/wire"
)

// PackageStats is ancestor/descendant aggregate fees and sizes for one mempool transaction (Core getmempoolentry shape).
type PackageStats struct {
	AncestorCount     int
	AncestorSize      int
	AncestorFeesKoinu int64
	DescendantCount   int
	DescendantSize    int
	DescendantFeesKoinu int64
}

// PackageStatsForTxID computes in-mempool ancestor and descendant packages including the seed tx.
// feesKoinu maps display txid to fee in koinu (missing entries count as 0).
// sizes maps display txid to serialized tx size in bytes.
func (p *Pool) PackageStatsForTxID(txid string, feesKoinu map[string]int64, sizes map[string]int) (PackageStats, error) {
	raw, err := p.GetRawByTxID(txid)
	if err != nil {
		return PackageStats{}, err
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return PackageStats{}, err
	}
	seedID := txidDisplayHex(tx.TxHash())
	anc, err := p.MempoolAncestorTxIDs(txid)
	if err != nil {
		return PackageStats{}, err
	}
	desc, err := p.MempoolDescendantTxIDs(txid)
	if err != nil {
		return PackageStats{}, err
	}
	pkgAnc := append(append([]string(nil), anc...), seedID)
	pkgDesc := append(append([]string(nil), desc...), seedID)

	var st PackageStats
	st.AncestorCount = len(pkgAnc)
	st.DescendantCount = len(pkgDesc)
	for _, id := range pkgAnc {
		st.AncestorFeesKoinu += feesKoinu[id]
		if sz, ok := sizes[id]; ok {
			st.AncestorSize += sz
		}
	}
	for _, id := range pkgDesc {
		st.DescendantFeesKoinu += feesKoinu[id]
		if sz, ok := sizes[id]; ok {
			st.DescendantSize += sz
		}
	}
	return st, nil
}

// AncestorPackageFeeSize returns ancestor-package aggregate fee and size (Core package min fee).
func (p *Pool) AncestorPackageFeeSize(txid string, feesKoinu map[string]int64, sizes map[string]int) (int64, int, error) {
	st, err := p.PackageStatsForTxID(txid, feesKoinu, sizes)
	if err != nil {
		return 0, 0, err
	}
	return st.AncestorFeesKoinu, st.AncestorSize, nil
}

// AncestorPackageSizes returns ancestor and descendant package serialized sizes for a pooled tx.
func (p *Pool) AncestorPackageSizes(txid string, feesKoinu map[string]int64, sizes map[string]int) (int, int, error) {
	st, err := p.PackageStatsForTxID(txid, feesKoinu, sizes)
	if err != nil {
		return 0, 0, err
	}
	return st.AncestorSize, st.DescendantSize, nil
}

// AdmissionAncestorFeesKoinu sums fees of in-mempool ancestors for a candidate tx.
func (p *Pool) AdmissionAncestorFeesKoinu(tx *wire.Tx, feesKoinu map[string]int64) (int64, error) {
	anc, err := p.admissionAncestorTxIDs(tx)
	if err != nil {
		return 0, err
	}
	var sum int64
	for _, id := range anc {
		sum += feesKoinu[id]
	}
	return sum, nil
}

// CheckAdmissionDescendantLimits rejects admission when an in-mempool ancestor would exceed descendant limits (Core CalculateMemPoolAncestors).
func (p *Pool) CheckAdmissionDescendantLimits(tx *wire.Tx, sizes map[string]int, maxDescendants int, maxDescendantKB int) error {
	if tx == nil {
		return nil
	}
	if maxDescendants <= 0 {
		maxDescendants = 25
	}
	maxDescBytes := maxDescendantKB * 1000
	if maxDescendantKB <= 0 {
		maxDescBytes = 101 * 1000
	}
	txSize := len(tx.SerializeForHash())
	anc, err := p.admissionAncestorTxIDs(tx)
	if err != nil {
		return err
	}
	emptyFees := map[string]int64{}
	for _, id := range anc {
		st, err := p.PackageStatsForTxID(id, emptyFees, sizes)
		if err != nil {
			continue
		}
		if st.DescendantCount+1 > maxDescendants {
			return fmt.Errorf("too-many-descendants for %s: %d+1 > %d", id, st.DescendantCount, maxDescendants)
		}
		if maxDescBytes > 0 && st.DescendantSize+txSize > maxDescBytes {
			return fmt.Errorf("too-long-mempool-chain: descendant package for %s would be %d bytes > %d", id, st.DescendantSize+txSize, maxDescBytes)
		}
	}
	return nil
}

// AdmissionPackageSizes returns package sizes for a tx not yet in the mempool (descendant package is the tx alone).
func (p *Pool) AdmissionPackageSizes(tx *wire.Tx, feesKoinu map[string]int64, sizes map[string]int) (int, int, error) {
	if tx == nil {
		return 0, 0, fmt.Errorf("mempool: nil transaction")
	}
	ancIDs, err := p.admissionAncestorTxIDs(tx)
	if err != nil {
		return 0, 0, err
	}
	seedSz := len(tx.SerializeForHash())
	ancSize := seedSz
	for _, id := range ancIDs {
		if sz, ok := sizes[id]; ok {
			ancSize += sz
		}
	}
	return ancSize, seedSz, nil
}

// BuildMempoolSizes returns serialized size per display txid in the pool.
func (p *Pool) BuildMempoolSizes() (map[string]int, error) {
	entries, err := p.sortedTxEntries()
	if err != nil {
		return nil, err
	}
	out := make(map[string]int, len(entries))
	for _, e := range entries {
		out[e.txid] = len(e.raw)
	}
	return out, nil
}
