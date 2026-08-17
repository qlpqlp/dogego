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

// WitnessScaleFactor scales legacy sigop counts to sigop cost (BIP141; no witness in DogeGo yet).
const WitnessScaleFactor = 4

// MaxBlockSigopsCost is consensus.h MAX_BLOCK_SIGOPS_COST.
const MaxBlockSigopsCost int64 = 80_000

// MaxStandardTxSigopsCost is policy.h MAX_STANDARD_TX_SIGOPS_COST.
const MaxStandardTxSigopsCost int64 = MaxBlockSigopsCost / 5

// ErrTxSigops is returned when a transaction exceeds standard sigop cost.
var ErrTxSigops = errors.New("consensus: transaction sigop cost too high")

// CountSigOps counts signature operations in a script (Core CScript::GetSigOpCount).
func CountSigOps(script []byte, accurate bool) int {
	const (
		opCheckSig            = 0xac
		opCheckSigVerify      = 0xad
		opCheckMultiSig       = 0xae
		opCheckMultiSigVerify = 0xaf
	)
	n := 0
	i := 0
	for i < len(script) {
		op := script[i]
		i++
		switch {
		case op >= 0x01 && op <= 0x4b:
			i += int(op)
		case op == 0x4c:
			if i >= len(script) {
				return n
			}
			ln := int(script[i])
			i += 1 + ln
		case op == 0x4d:
			if i+1 >= len(script) {
				return n
			}
			ln := int(script[i]) | int(script[i+1])<<8
			i += 2 + ln
		case op == 0x4e:
			if i+3 >= len(script) {
				return n
			}
			ln := int(script[i]) | int(script[i+1])<<8 | int(script[i+2])<<16 | int(script[i+3])<<24
			i += 4 + ln
		case op >= 0x51 && op <= 0x60:
		case op == opCheckSig, op == opCheckSigVerify:
			n++
		case op == opCheckMultiSig, op == opCheckMultiSigVerify:
			if accurate && i > 1 {
				j := i - 2
				if script[j] >= 0x51 && script[j] <= 0x60 {
					n += int(script[j] - 0x50)
				} else {
					n += 20
				}
			} else {
				n += 20
			}
		}
	}
	return n
}

// GetLegacySigOpCount sums sigops in inputs and outputs (Core GetLegacySigOpCount).
func GetLegacySigOpCount(tx *wire.Tx) int {
	if tx == nil {
		return 0
	}
	n := 0
	for i := range tx.Vin {
		n += CountSigOps(tx.Vin[i].Script, false)
	}
	for i := range tx.Vout {
		n += CountSigOps(tx.Vout[i].PkScript, false)
	}
	return n
}

// GetP2SHSigOpCount counts sigops in P2SH redeem scripts revealed by inputs.
func GetP2SHSigOpCount(tx *wire.Tx, view PrevOutView) int {
	if tx == nil || IsCoinbaseTx(tx) || view == nil {
		return 0
	}
	n := 0
	for i := range tx.Vin {
		prev, ok := view.Lookup(tx.Vin[i].PrevHash, tx.Vin[i].PrevIdx)
		if !ok || ClassifyScriptPubKey(prev.PkScript) != ScriptScriptHash {
			continue
		}
		redeem, err := lastScriptPush(tx.Vin[i].Script)
		if err != nil {
			continue
		}
		n += CountSigOps(redeem, true)
	}
	return n
}

// GetTransactionSigOpCost returns scaled sigop cost for legacy txs (no witness).
func GetTransactionSigOpCost(tx *wire.Tx, view PrevOutView) int64 {
	legacy := int64(GetLegacySigOpCount(tx))
	if !IsCoinbaseTx(tx) && view != nil {
		legacy += int64(GetP2SHSigOpCount(tx, view))
	}
	return legacy * WitnessScaleFactor
}

// CheckTxSigOpCost rejects txs above standard relay sigop cost (mempool policy).
func CheckTxSigOpCost(tx *wire.Tx, view PrevOutView) error {
	if tx == nil {
		return nil
	}
	if cost := GetTransactionSigOpCost(tx, view); cost > MaxStandardTxSigopsCost {
		return fmt.Errorf("%w: %d > %d", ErrTxSigops, cost, MaxStandardTxSigopsCost)
	}
	return nil
}

// CheckBlockSigOpCostRaw scans a block payload for aggregate sigop cost without retaining all txs.
func CheckBlockSigOpCostRaw(blockRaw []byte, view PrevOutView) error {
	if len(blockRaw) < 80 || view == nil {
		return nil
	}
	intra := &blockUndoView{}
	chainView := MultiPrevOutView{intra, view}
	var total int64
	return wire.ForEachBlockTx(blockRaw, func(i uint32, tx *wire.Tx) error {
		if i == 0 {
			intra.addTx(tx, 0)
			return nil
		}
		cost := GetTransactionSigOpCost(tx, chainView)
		if total+cost > MaxBlockSigopsCost {
			return fmt.Errorf("bad-blk-sigops: %d+%d > %d", total, cost, MaxBlockSigopsCost)
		}
		total += cost
		intra.addTx(tx, 0)
		return nil
	})
}

// CheckBlockSigOpCost rejects blocks whose total transaction sigop cost is too high.
func CheckBlockSigOpCost(pb *wire.ParsedBlock, view PrevOutView) error {
	if pb == nil {
		return nil
	}
	intra := &blockUndoView{}
	chainView := MultiPrevOutView{intra, view}
	var total int64
	for i, tx := range pb.Txs {
		if i == 0 {
			intra.addTx(tx, 0)
			continue
		}
		cost := GetTransactionSigOpCost(tx, chainView)
		if total+cost > MaxBlockSigopsCost {
			return fmt.Errorf("bad-blk-sigops: %d+%d > %d", total, cost, MaxBlockSigopsCost)
		}
		total += cost
		intra.addTx(tx, 0)
	}
	return nil
}
