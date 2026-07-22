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

// ErrMempoolCoinbase is returned when a transaction uses a null prevout (coinbase-style input).
var ErrMempoolCoinbase = errors.New("consensus: coinbase transactions are not accepted into the mempool")

// AcceptMempoolTx applies mempool admission with optional view-only policy (no spend conflict checks).
func AcceptMempoolTx(tx *wire.Tx, view PrevOutView) error {
	return AcceptMempoolTxAdmission(tx, MempoolAdmission{View: view})
}

// AcceptMempoolTxAdmission runs CheckTransaction, spend-conflict checks, and script verification.
func AcceptMempoolTxAdmission(tx *wire.Tx, adm MempoolAdmission) error {
	return acceptMempoolTx(tx, adm, true)
}

func acceptMempoolTx(tx *wire.Tx, adm MempoolAdmission, verifyScript bool) error {
	if tx == nil {
		return fmt.Errorf("consensus: nil transaction")
	}
	if IsCoinbaseTx(tx) {
		return ErrMempoolCoinbase
	}
	if err := CheckTransaction(tx, true); err != nil {
		return err
	}
	if err := adm.CheckStandard(tx); err != nil {
		return err
	}
	if err := adm.CheckSpendConflicts(tx); err != nil {
		return err
	}
	if pkg, ok := adm.Pool.(MempoolPackagePool); ok {
		var sizes map[string]int
		if ap, ok := adm.Pool.(MempoolAncestorPackagePool); ok {
			sizes, _ = ap.BuildMempoolSizes()
		}
		if err := CheckMempoolPackageLimits(tx, pkg, sizes, adm.MaxMempoolAncestors, adm.MaxMempoolDescendants, adm.MaxMempoolDescendantSizeKB); err != nil {
			return err
		}
	}
	if pkg, ok := adm.Pool.(MempoolAncestorPackagePool); ok {
		if err := CheckMempoolPackageSizeLimits(tx, pkg, adm.View, adm.MaxMempoolAncestorSizeKB, adm.MaxMempoolDescendantSizeKB); err != nil {
			return err
		}
	}
	if !adm.SkipMinRelayFee {
		feePerKB := adm.MinRelayFeePerKB
		if feePerKB == 0 {
			feePerKB = MinRelayTxFeePerKB()
		}
		if pkg, ok := adm.Pool.(MempoolAncestorPackagePool); ok {
			if err := CheckMinRelayFeeMempool(tx, adm.View, pkg, feePerKB); err != nil {
				return err
			}
		} else if err := CheckMinRelayFee(tx, adm.View, feePerKB); err != nil {
			return err
		}
	}
	maxAbsurd := adm.MaxAbsurdFeeKoinu
	if maxAbsurd == 0 {
		maxAbsurd = DefaultMaxAbsurdTxFeeKoinu
	} else if maxAbsurd < 0 {
		maxAbsurd = 0
	}
	if err := CheckAbsurdTxFee(tx, adm.View, maxAbsurd); err != nil {
		return err
	}
	if err := adm.CheckCoinbaseMaturity(tx); err != nil {
		return err
	}
	if err := adm.CheckFinal(tx); err != nil {
		return err
	}
	if err := adm.CheckSequenceLocks(tx); err != nil {
		return err
	}
	if err := adm.CheckSigOpCost(tx); err != nil {
		return err
	}
	if !verifyScript {
		return nil
	}
	spendHeight := int64(0)
	if adm.Journal != nil {
		if tip, err := adm.Journal.TipHeight(); err == nil {
			spendHeight = tip + 1
		}
	}
	return VerifyScriptFlags(tx, adm.View, ScriptFlagsForMempool(spendHeight, adm.Net, adm.Journal))
}

// CheckFinal rejects transactions not final for the next block (Core CheckFinalTx / non-final).
func (a MempoolAdmission) CheckFinal(tx *wire.Tx) error {
	if a.Journal == nil {
		return nil
	}
	ctx, err := LockTimeContextForNextBlock(a.Journal, true)
	if err != nil {
		return err
	}
	return CheckTxFinal(tx, ctx)
}

// CheckSequenceLocks rejects txs that fail BIP68 relative lock-times for the next block.
func (a MempoolAdmission) CheckSequenceLocks(tx *wire.Tx) error {
	if a.Journal == nil {
		return nil
	}
	if !PrevHeightsResolvableForSequenceLocks(tx, a.Index, a.View) {
		return nil
	}
	tip, err := a.Journal.TipHeight()
	if err != nil {
		return err
	}
	next := int(tip + 1)
	prevH, err := PrevHeightsForTx(tx, a.Index, a.Journal, tip+1, nil, next, a.View)
	if err != nil {
		return err
	}
	return CheckTxSequenceLocks(tx, SequenceEvalBlock{Height: tip + 1}, prevH, a.Journal, a.Net)
}

// CheckCoinbaseMaturity rejects immature coinbase spends at the next block height.
func (a MempoolAdmission) CheckCoinbaseMaturity(tx *wire.Tx) error {
	if a.Journal == nil || a.Index == nil {
		return nil
	}
	tip, err := a.Journal.TipHeight()
	if err != nil {
		return err
	}
	return CheckTxCoinbaseMaturity(tx, tip+1, a.Net, a.Index, a.Journal)
}

// CheckSigOpCost applies standard MAX_STANDARD_TX_SIGOPS_COST relay policy.
func (a MempoolAdmission) CheckSigOpCost(tx *wire.Tx) error {
	if a.View == nil {
		return nil
	}
	return CheckTxSigOpCost(tx, a.View)
}
