// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"fmt"

	"dogego/wire"
)

// ErrAbsurdlyHighFee is returned when a transaction fee exceeds the configured absurd limit.
var ErrAbsurdlyHighFee = errors.New("consensus: absurdly-high-fee")

// DefaultMaxAbsurdTxFeeKoinu matches Core DEFAULT_TRANSACTION_MAXFEE (RECOMMENDED_MIN_TX_FEE * 10000).
const DefaultMaxAbsurdTxFeeKoinu int64 = RecommendedMinTxFee * 10000

// MempoolAncestorPackagePool reports ancestor-package fee and size for min relay checks.
type MempoolAncestorPackagePool interface {
	MempoolPackagePool
	RawMemPoolTxIDs() ([]string, error)
	GetRawByTxID(rpcTxid string) ([]byte, error)
	BuildMempoolSizes() (map[string]int, error)
	AncestorPackageFeeSize(displayTxid string, feesKoinu map[string]int64, sizes map[string]int) (fee int64, size int, err error)
	AncestorPackageSizes(displayTxid string, feesKoinu map[string]int64, sizes map[string]int) (ancestorSize, descendantSize int, err error)
	AdmissionPackageSizes(tx *wire.Tx, feesKoinu map[string]int64, sizes map[string]int) (ancestorSize, descendantSize int, err error)
}

// CheckMinRelayFeeMempool applies min relay fee to the tx or its in-mempool ancestor package.
func CheckMinRelayFeeMempool(tx *wire.Tx, view PrevOutView, pool MempoolAncestorPackagePool, feePerKB uint64) error {
	if feePerKB == 0 || tx == nil {
		return nil
	}
	fee, err := TxFee(tx, view)
	if err != nil {
		return err
	}
	sz := len(tx.SerializeForHash())
	if pool != nil {
		fees := BuildMempoolFeesKoinuFromPool(pool, view)
		sizes, _ := pool.BuildMempoolSizes()
		id := txidDisplayFromLE(tx.TxHash())
		if af, asz, err := pool.AncestorPackageFeeSize(id, fees, sizes); err == nil && asz > 0 {
			fee = af
			sz = asz
		}
	}
	need := FeeForSize(feePerKB, sz)
	if fee < need {
		return fmt.Errorf("%w: have %d need %d for %d bytes", ErrMinRelayFee, fee, need, sz)
	}
	return nil
}

// packageOverlayView exposes outputs of earlier txs in a submitpackage batch.
type packageOverlayView struct {
	outs map[[36]byte]PrevOut
}

func (v *packageOverlayView) Lookup(prevHash [32]byte, idx uint32) (PrevOut, bool) {
	if v == nil || v.outs == nil {
		return PrevOut{}, false
	}
	o, ok := v.outs[outpointKey(prevHash, idx)]
	return o, ok
}

func (v *packageOverlayView) addTx(tx *wire.Tx) {
	if v == nil || tx == nil {
		return
	}
	if v.outs == nil {
		v.outs = make(map[[36]byte]PrevOut)
	}
	h := tx.TxHash()
	for i, o := range tx.Vout {
		v.outs[outpointKey(h, uint32(i))] = PrevOut{
			Value:    o.Value,
			PkScript: append([]byte(nil), o.PkScript...),
		}
	}
}

// PackageTxsFeeSize sums fees and serialized sizes for a topologically ordered package.
// Earlier package outputs are available when computing later tx fees (CPFP).
func PackageTxsFeeSize(txs []*wire.Tx, view PrevOutView) (fee int64, size int, err error) {
	overlay := &packageOverlayView{}
	for _, tx := range txs {
		if tx == nil {
			continue
		}
		v := PrevOutView(overlay)
		if view != nil {
			v = MultiPrevOutView{overlay, view}
		}
		f, ferr := TxFee(tx, v)
		if ferr != nil {
			return 0, 0, ferr
		}
		fee += f
		size += len(tx.SerializeForHash())
		overlay.addTx(tx)
	}
	return fee, size, nil
}

// CheckMinRelayFeePackageTxs applies min relay to the package as a unit (Core submitpackage CPFP).
func CheckMinRelayFeePackageTxs(txs []*wire.Tx, view PrevOutView, feePerKB uint64) error {
	if feePerKB == 0 || len(txs) == 0 {
		return nil
	}
	fee, size, err := PackageTxsFeeSize(txs, view)
	if err != nil {
		return err
	}
	need := FeeForSize(feePerKB, size)
	if fee < need {
		return fmt.Errorf("%w: have %d need %d for %d bytes", ErrMinRelayFee, fee, need, size)
	}
	return nil
}

// CheckAbsurdTxFee rejects fees above maxFee when maxFee > 0 (Core nAbsurdFee).
func CheckAbsurdTxFee(tx *wire.Tx, view PrevOutView, maxFee int64) error {
	if maxFee <= 0 || tx == nil {
		return nil
	}
	fee, err := TxFee(tx, view)
	if err != nil {
		return err
	}
	if fee > maxFee {
		return fmt.Errorf("%w: %d > %d", ErrAbsurdlyHighFee, fee, maxFee)
	}
	return nil
}

// BuildMempoolFeesKoinuFromPool builds fee map using pool raw tx access.
func BuildMempoolFeesKoinuFromPool(pool MempoolAncestorPackagePool, view PrevOutView) map[string]int64 {
	out := make(map[string]int64)
	if pool == nil || view == nil {
		return out
	}
	ids, err := pool.RawMemPoolTxIDs()
	if err != nil {
		return out
	}
	for _, id := range ids {
		raw, err := pool.GetRawByTxID(id)
		if err != nil {
			continue
		}
		tx, err := wire.DeserializeTx(raw)
		if err != nil {
			continue
		}
		fee, err := TxFee(tx, view)
		if err != nil || fee < 0 {
			continue
		}
		out[id] = fee
	}
	return out
}
