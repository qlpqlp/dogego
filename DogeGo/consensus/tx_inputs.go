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

// CheckTxInputs verifies prevouts exist and input values cover outputs (Core Consensus::CheckTxInputs).
// Coinbase maturity is enforced separately via CheckTxCoinbaseMaturity.
func CheckTxInputs(tx *wire.Tx, view PrevOutView) error {
	if tx == nil || IsCoinbaseTx(tx) {
		return nil
	}
	if view == nil {
		return fmt.Errorf("bad-txns: inputs unavailable")
	}
	var inSum int64
	for i := range tx.Vin {
		prev, ok := view.Lookup(tx.Vin[i].PrevHash, tx.Vin[i].PrevIdx)
		if !ok {
			return fmt.Errorf("bad-txns: inputs unavailable")
		}
		if prev.Value < 0 || prev.Value > MaxMoney {
			return fmt.Errorf("bad-txns-inputvalues-outofrange")
		}
		inSum += prev.Value
		if inSum < 0 || inSum > MaxMoney {
			return fmt.Errorf("bad-txns-inputvalues-outofrange")
		}
	}
	var outSum int64
	for _, o := range tx.Vout {
		outSum += o.Value
	}
	if inSum < outSum {
		return fmt.Errorf("bad-txns-in-belowout")
	}
	fee := inSum - outSum
	if fee < 0 {
		return fmt.Errorf("bad-txns-fee-negative")
	}
	return nil
}
