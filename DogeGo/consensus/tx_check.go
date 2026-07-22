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

// MaxMoney is Dogecoin Core MAX_MONEY (amount.h).
const MaxMoney = 10_000_000_000 * KoinuPerCoin

// MaxBlockBaseSize is consensus.h MAX_BLOCK_BASE_SIZE.
const MaxBlockBaseSize = 1_000_000

// CheckTransaction applies context-free checks from Core CheckTransaction.
func CheckTransaction(tx *wire.Tx, checkDuplicateInputs bool) error {
	if tx == nil {
		return fmt.Errorf("bad-txns: nil transaction")
	}
	if len(tx.Vin) == 0 {
		return fmt.Errorf("bad-txns-vin-empty")
	}
	if len(tx.Vout) == 0 {
		return fmt.Errorf("bad-txns-vout-empty")
	}
	if len(tx.SerializeForHash()) > MaxBlockBaseSize {
		return fmt.Errorf("bad-txns-oversize")
	}
	if tx.HasWitness() {
		return fmt.Errorf("bad-txns-witness-not-supported")
	}
	var totalOut int64
	for i, o := range tx.Vout {
		if len(o.PkScript) == 0 {
			return fmt.Errorf("bad-txns-vout-empty-scriptpubkey")
		}
		if IsUnspendableScript(o.PkScript) && o.Value != 0 {
			return fmt.Errorf("bad-txns-unspendable-output")
		}
		if o.Value < 0 {
			return fmt.Errorf("bad-txns-vout-negative")
		}
		if o.Value > MaxMoney {
			return fmt.Errorf("bad-txns-vout-toolarge")
		}
		totalOut += o.Value
		if totalOut < 0 || totalOut > MaxMoney {
			return fmt.Errorf("bad-txns-txouttotal-toolarge")
		}
		if len(o.PkScript) > MaxBlockBaseSize {
			return fmt.Errorf("bad-txns-vout-script-toolarge at %d", i)
		}
	}
	if checkDuplicateInputs {
		seen := make(map[[36]byte]struct{}, len(tx.Vin))
		for _, in := range tx.Vin {
			var k [36]byte
			copy(k[:32], in.PrevHash[:])
			copy(k[32:], []byte{byte(in.PrevIdx), byte(in.PrevIdx >> 8), byte(in.PrevIdx >> 16), byte(in.PrevIdx >> 24)})
			if _, ok := seen[k]; ok {
				return fmt.Errorf("bad-txns-inputs-duplicate")
			}
			seen[k] = struct{}{}
		}
	}
	if IsCoinbaseTx(tx) {
		if len(tx.Vin[0].Script) < 2 || len(tx.Vin[0].Script) > 100 {
			return fmt.Errorf("bad-cb-length")
		}
	} else {
		for i := range tx.Vin {
			if IsNullOutpoint(&tx.Vin[i]) {
				return fmt.Errorf("bad-txns-prevout-null")
			}
		}
	}
	return nil
}

// IsCoinbaseTx reports whether tx is a coinbase (single null prevout).
func IsCoinbaseTx(tx *wire.Tx) bool {
	return len(tx.Vin) == 1 && IsNullOutpoint(&tx.Vin[0])
}

// IsNullOutpoint reports the coinbase null prevout (Core COutPoint::IsNull).
func IsNullOutpoint(in *wire.TxIn) bool {
	if in == nil {
		return false
	}
	var z [32]byte
	return in.PrevHash == z && in.PrevIdx == 0xffffffff
}
