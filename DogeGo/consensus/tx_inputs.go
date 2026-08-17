// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"

	"dogego/chain"
	"dogego/wire"
)

// CheckTxInputs verifies prevouts exist and input values cover outputs (Core Consensus::CheckTxInputs).
func CheckTxInputs(tx *wire.Tx, view PrevOutView) error {
	return checkTxInputsAtHeight(tx, view, 0, 0, false)
}

// CheckTxInputsAtHeight is ConnectBlock's CheckTxInputs: values, fees, and coinbase maturity from the UTXO view.
func CheckTxInputsAtHeight(tx *wire.Tx, view PrevOutView, spendHeight int64, net chain.Network) error {
	return checkTxInputsAtHeight(tx, view, spendHeight, net, true)
}

func checkTxInputsAtHeight(tx *wire.Tx, view PrevOutView, spendHeight int64, net chain.Network, enforceMaturity bool) error {
	if tx == nil || IsCoinbaseTx(tx) {
		return nil
	}
	if view == nil {
		return fmt.Errorf("bad-txns: inputs unavailable")
	}
	maturity := int64(0)
	if enforceMaturity && spendHeight > 0 {
		m := LookupConsensus(net, spendHeight).CoinbaseMaturity
		if m > 0 {
			maturity = int64(m)
		} else {
			maturity = 30
		}
	}
	var inSum int64
	for i := range tx.Vin {
		prev, ok := view.Lookup(tx.Vin[i].PrevHash, tx.Vin[i].PrevIdx)
		if !ok {
			return fmt.Errorf("bad-txns: inputs unavailable")
		}
		if prev.Coinbase && maturity > 0 {
			if spendHeight-prev.Height < maturity {
				return fmt.Errorf("%w (input %d, need %d blocks, have %d)", ErrCoinbaseImmature, i, maturity, spendHeight-prev.Height)
			}
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
