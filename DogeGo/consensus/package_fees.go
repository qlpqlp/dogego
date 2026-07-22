// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"dogego/mempool"
	"dogego/wire"
)

// PackageFeeReport holds mempool package fee fields for RPC (testmempoolaccept / getmempoolentry).
type PackageFeeReport struct {
	BaseFeeKoinu       int64
	AncestorFeeKoinu   int64
	DescendantFeeKoinu int64
	EffectiveRatePerKB int64 // koinu per kB for ancestor package
}

// PackageFeeReportForTx computes base and in-mempool package fees for a candidate tx.
func PackageFeeReportForTx(tx *wire.Tx, pool *mempool.Pool, view PrevOutView) (PackageFeeReport, error) {
	var out PackageFeeReport
	if tx == nil {
		return out, nil
	}
	base, err := TxFee(tx, view)
	if err != nil {
		return out, err
	}
	out.BaseFeeKoinu = base
	out.AncestorFeeKoinu = base
	out.DescendantFeeKoinu = base
	sz := len(tx.SerializeForHash())
	if sz > 0 {
		out.EffectiveRatePerKB = base * 1000 / int64(sz)
	}
	if pool == nil {
		return out, nil
	}
	sizes, _ := pool.BuildMempoolSizes()
	fees := BuildMempoolFeesKoinuFromPool(pool, view)
	id := txidDisplayFromLE(tx.TxHash())
	if st, err := pool.PackageStatsForTxID(id, fees, sizes); err == nil {
		out.AncestorFeeKoinu = st.AncestorFeesKoinu
		out.DescendantFeeKoinu = st.DescendantFeesKoinu
		if st.AncestorSize > 0 {
			out.EffectiveRatePerKB = st.AncestorFeesKoinu * 1000 / int64(st.AncestorSize)
		}
		return out, nil
	}
	ancFees, _ := pool.AdmissionAncestorFeesKoinu(tx, fees)
	out.AncestorFeeKoinu = ancFees + base
	if _, asz, err := pool.AdmissionPackageSizes(tx, fees, sizes); err == nil && asz > 0 {
		out.EffectiveRatePerKB = out.AncestorFeeKoinu * 1000 / int64(asz)
	}
	return out, nil
}
