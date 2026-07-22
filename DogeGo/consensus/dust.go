// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "dogego/wire"

// DustRelayFeePerKB default matches Core dustRelayFee (typically min relay rate).
const DustRelayFeePerKB = DefaultMinRelayTxFeePerKB

// DustThresholdForOutput returns the fee-based dust threshold (3× cost to spend the output).
func DustThresholdForOutput(out wire.TxOut, dustRelayPerKB uint64) int64 {
	if isNullDataScript(out.PkScript) {
		return 0
	}
	if dustRelayPerKB == 0 {
		dustRelayPerKB = MinRelayTxFeePerKB()
	}
	nSize := txOutSerializeSize(out)
	return 3 * FeeForSize(dustRelayPerKB, nSize)
}

func txOutSerializeSize(out wire.TxOut) int {
	// Value (8) + compact size + script bytes (Core GetSerializeSize SER_DISK).
	n := 8 + 1 + len(out.PkScript)
	if len(out.PkScript) >= 0xfd {
		n = 8 + 3 + len(out.PkScript)
	}
	return n
}

// EffectiveDustLimit returns max(hard dust limit, fee-based threshold).
func EffectiveDustLimit(out wire.TxOut, pol StandardPolicy, dustRelayPerKB uint64) int64 {
	hard := pol.HardDustLimitKoinu
	if hard <= 0 {
		hard = HardDustLimitKoinu
	}
	soft := DustThresholdForOutput(out, dustRelayPerKB)
	if soft > hard {
		return soft
	}
	return hard
}

// IsOutputDustEffective applies hard and fee-based dust limits.
func IsOutputDustEffective(out wire.TxOut, pol StandardPolicy, dustRelayPerKB uint64) bool {
	if isNullDataScript(out.PkScript) {
		return false
	}
	return out.Value < EffectiveDustLimit(out, pol, dustRelayPerKB)
}
