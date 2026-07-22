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

// DefaultMinRelayTxFeePerKB matches Core DEFAULT_MIN_RELAY_TX_FEE (RECOMMENDED_MIN_TX_FEE / 10).
const DefaultMinRelayTxFeePerKB = KoinuPerCoin / 1000

// DefaultIncrementalRelayFeePerKB matches Core DEFAULT_INCREMENTAL_RELAY_FEE for BIP125 replacements.
const DefaultIncrementalRelayFeePerKB = DefaultMinRelayTxFeePerKB

// ErrMinRelayFee is returned when a transaction fee is below the minimum relay rate.
var ErrMinRelayFee = errors.New("consensus: transaction fee below minimum relay fee")

// EffectiveMinRelayFeePerKB returns max(configured min relay, peer feefilter, mempool rolling minimum) in koinu per kB.
func EffectiveMinRelayFeePerKB(peerKoinuPerKB, mempoolRollingKoinuPerKB uint64) uint64 {
	out := MinRelayTxFeePerKB()
	if peerKoinuPerKB > out {
		out = peerKoinuPerKB
	}
	if mempoolRollingKoinuPerKB > out {
		out = mempoolRollingKoinuPerKB
	}
	return out
}

// FeeForSize computes the minimum fee for nBytes at feePerKB (Core CFeeRate::GetFee).
func FeeForSize(feePerKB uint64, nBytes int) int64 {
	if feePerKB == 0 || nBytes <= 0 {
		return 0
	}
	fee := int64(feePerKB) * int64(nBytes) / 1000
	if fee == 0 {
		fee = 1
	}
	return fee
}

// TxFee returns input sum minus output sum using view (Core GetTransactionFee).
func TxFee(tx *wire.Tx, view PrevOutView) (int64, error) {
	if tx == nil || view == nil {
		return 0, fmt.Errorf("bad-txns: nil tx or view")
	}
	var inSum, outSum int64
	for _, o := range tx.Vout {
		outSum += o.Value
	}
	for i := range tx.Vin {
		prev, ok := view.Lookup(tx.Vin[i].PrevHash, tx.Vin[i].PrevIdx)
		if !ok {
			return 0, fmt.Errorf("%w (input %d)", ErrMissingPrevout, i)
		}
		inSum += prev.Value
	}
	if inSum < outSum {
		return 0, fmt.Errorf("bad-txns-in-belowout")
	}
	return inSum - outSum, nil
}

// TxFeeRateKoinuPerKB returns the transaction fee rate in koinu per kB (false when fee unknown).
func TxFeeRateKoinuPerKB(tx *wire.Tx, raw []byte, view PrevOutView) (uint64, bool) {
	if tx == nil || view == nil {
		return 0, false
	}
	fee, err := TxFee(tx, view)
	if err != nil || fee < 0 {
		return 0, false
	}
	sz := len(raw)
	if sz <= 0 {
		sz = len(tx.SerializeForHash())
	}
	if sz <= 0 {
		return 0, false
	}
	return uint64(fee * 1000 / int64(sz)), true
}

// MeetsPeerFeeFilter reports whether feeRate meets the peer's BIP133 feefilter (0 = no filter).
func MeetsPeerFeeFilter(feeRateKoinuPerKB, peerFilter uint64) bool {
	if peerFilter == 0 {
		return true
	}
	return feeRateKoinuPerKB >= peerFilter
}

// CheckMinRelayFee rejects txs whose fee is below feePerKB (0 disables the check).
func CheckMinRelayFee(tx *wire.Tx, view PrevOutView, feePerKB uint64) error {
	if feePerKB == 0 || tx == nil {
		return nil
	}
	fee, err := TxFee(tx, view)
	if err != nil {
		return err
	}
	need := FeeForSize(feePerKB, len(tx.SerializeForHash()))
	if fee < need {
		return fmt.Errorf("%w: have %d need %d for %d bytes", ErrMinRelayFee, fee, need, len(tx.SerializeForHash()))
	}
	return nil
}
