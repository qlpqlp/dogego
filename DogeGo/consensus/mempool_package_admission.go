// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"

	"dogego/wire"
)

// MempoolPackagePool reports ancestor/descendant counts for package limit checks.
type MempoolPackagePool interface {
	MempoolDescendantCount(displayTxid string) (int, error)
	MempoolAncestorCount(displayTxid string) (int, error)
	AdmissionAncestorCount(tx *wire.Tx) (int, error)
	// CheckAdmissionDescendantLimits ensures no in-mempool ancestor would exceed descendant limits after admitting tx.
	CheckAdmissionDescendantLimits(tx *wire.Tx, sizes map[string]int, maxDescendants int, maxDescendantKB int) error
}

// CheckMempoolPackageLimits rejects txs whose in-mempool ancestor or descendant count exceeds defaults.
func CheckMempoolPackageLimits(tx *wire.Tx, pool MempoolPackagePool, sizes map[string]int, maxAncestors, maxDescendants, maxDescendantKB int) error {
	if tx == nil || pool == nil {
		return nil
	}
	if maxAncestors <= 0 {
		maxAncestors = DefaultMaxMempoolAncestors
	}
	if maxDescendants <= 0 {
		maxDescendants = DefaultMaxMempoolDescendants
	}
	if maxDescendantKB <= 0 {
		maxDescendantKB = DefaultMaxMempoolDescendantSizeKB
	}
	id := txidDisplayFromLE(tx.TxHash())
	ancN, ancErr := pool.MempoolAncestorCount(id)
	if ancErr != nil {
		if n, err := pool.AdmissionAncestorCount(tx); err == nil {
			ancN = n
			ancErr = nil
		}
	}
	if ancErr == nil && ancN > maxAncestors {
		return fmt.Errorf("too-long-mempool-chain: %d ancestors > %d", ancN, maxAncestors)
	}
	descN, descErr := pool.MempoolDescendantCount(id)
	if descErr == nil && descN > maxDescendants {
		return fmt.Errorf("too-many-descendants: %d > %d", descN, maxDescendants)
	}
	if err := pool.CheckAdmissionDescendantLimits(tx, sizes, maxDescendants, maxDescendantKB); err != nil {
		return err
	}
	return nil
}

// CheckMempoolPackageSizeLimits rejects txs whose ancestor or descendant package size exceeds Core byte limits.
func CheckMempoolPackageSizeLimits(tx *wire.Tx, pool MempoolAncestorPackagePool, view PrevOutView, maxAncestorKB, maxDescendantKB int) error {
	if tx == nil || pool == nil {
		return nil
	}
	if maxAncestorKB <= 0 {
		maxAncestorKB = DefaultMaxMempoolAncestorSizeKB
	}
	if maxDescendantKB <= 0 {
		maxDescendantKB = DefaultMaxMempoolDescendantSizeKB
	}
	maxAncBytes := maxAncestorKB * 1000
	maxDescBytes := maxDescendantKB * 1000
	fees := BuildMempoolFeesKoinuFromPool(pool, view)
	sizes, _ := pool.BuildMempoolSizes()
	if mp, ok := pool.(MempoolPackagePool); ok {
		if err := mp.CheckAdmissionDescendantLimits(tx, sizes, 0, maxDescendantKB); err != nil {
			return err
		}
	}
	id := txidDisplayFromLE(tx.TxHash())
	ancSize := len(tx.SerializeForHash())
	descSize := ancSize
	if a, d, err := pool.AncestorPackageSizes(id, fees, sizes); err == nil {
		ancSize, descSize = a, d
	} else if a, d, err := pool.AdmissionPackageSizes(tx, fees, sizes); err == nil {
		ancSize, descSize = a, d
	}
	if ancSize > maxAncBytes {
		return fmt.Errorf("too-long-mempool-chain: ancestor package %d bytes > %d", ancSize, maxAncBytes)
	}
	if descSize > maxDescBytes {
		return fmt.Errorf("too-long-mempool-chain: descendant package %d bytes > %d", descSize, maxDescBytes)
	}
	return nil
}
