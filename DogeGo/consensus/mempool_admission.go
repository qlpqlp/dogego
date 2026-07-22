// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"fmt"

	"dogego/chain"
	"dogego/wire"
)

// ErrSpendInMempool is returned when an input spends an outpoint already used by a pooled tx.
var ErrSpendInMempool = errors.New("consensus: outpoint already spent in mempool")

// ErrSpendOnChain is returned when an input spends an outpoint already spent in a stored block.
var ErrSpendOnChain = errors.New("consensus: outpoint already spent on chain")

// MempoolConflictPool reports in-mempool double spends (BIP133-style pool conflict).
type MempoolConflictPool interface {
	SpendsOutpoint(rpcTxid string, vout uint32) bool
}

// chainSpendChecker detects whether a prevout is already spent on the confirmed chain.
type chainSpendChecker interface {
	OutpointSpent(prevHash [32]byte, vout uint32) (bool, error)
}

// MempoolAdmission groups prevout resolution and spend-conflict checks for AcceptMempoolTx.
type MempoolAdmission struct {
	View             PrevOutView
	Chain            chainSpendChecker
	Journal          HeaderChain
	Index            TxIndexer
	Pool             MempoolConflictPool
	RBFPool          MempoolRBFPool // optional; enables BIP125 replacement before conflict rejection
	FullRBF          bool           // when true, allow replacement of non-BIP125-signaling conflicts (mempoolfullrbf)
	MinRelayFeePerKB uint64         // koinu per kB; 0 uses EffectiveMinRelayFeePerKB(0)
	SkipMinRelayFee  bool           // when true, skip per-tx min relay (caller validated package feerate)
	Net              chain.Network  // chain for maturity and script flags
	Standard         StandardPolicy // relay standardness; zero value uses DefaultStandardPolicy()
	MaxAbsurdFeeKoinu int64 // 0 uses DefaultMaxAbsurdTxFeeKoinu; negative disables absurd-fee check
	// Package limits (0 = Core defaults).
	MaxMempoolAncestors         int
	MaxMempoolDescendants       int
	MaxMempoolAncestorSizeKB    int
	MaxMempoolDescendantSizeKB  int
}

// NewMempoolAdmission builds full-node admission policy: mempool + chain prevouts and conflict checks.
func NewMempoolAdmission(pool MempoolPool, conflict MempoolConflictPool, index TxIndexer, raw RawBlockGetter, journal HeaderChain, net chain.Network) MempoolAdmission {
	return NewMempoolAdmissionWithUtxo(pool, conflict, nil, index, raw, journal, net)
}

// NewMempoolAdmissionWithUtxo is like NewMempoolAdmission but uses the UTXO cache for prevouts and spend checks when set.
func NewMempoolAdmissionWithUtxo(pool MempoolPool, conflict MempoolConflictPool, utxo UtxoOutpointSource, index TxIndexer, raw RawBlockGetter, journal HeaderChain, net chain.Network) MempoolAdmission {
	adm := MempoolAdmission{
		View:    AdmissionPrevOutViewWithUtxo(pool, utxo, index, raw),
		Chain:   NewUtxoChainSpendView(utxo, journal, raw, index),
		Journal: journal,
		Index:   index,
		Pool:    conflict,
		Net:     net,
	}
	if rbf, ok := conflict.(MempoolRBFPool); ok {
		adm.RBFPool = rbf
	}
	return adm
}

// CheckSpendConflicts rejects inputs that double-spend in the mempool or on stored chain blocks.
// When RBFPool is set, BIP125-signaling conflicts may be removed if the candidate pays a higher fee rate.
func (a MempoolAdmission) CheckSpendConflicts(tx *wire.Tx) error {
	if tx == nil {
		return fmt.Errorf("consensus: nil transaction")
	}
	err := a.checkSpendConflictsOnce(tx)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrSpendInMempool) || a.RBFPool == nil {
		return err
	}
	if rbfErr := TryResolveMempoolRBFConflicts(tx, a.RBFPool, a.View, a.FullRBF); rbfErr != nil {
		if errors.Is(rbfErr, ErrRBFInsufficientFee) || errors.Is(rbfErr, ErrRBFNotReplaceable) ||
			errors.Is(rbfErr, ErrRBFTxTooManyDescendants) || errors.Is(rbfErr, ErrRBFTooManyConflicts) ||
			errors.Is(rbfErr, ErrRBFNewUnconfirmedInput) {
			return rbfErr
		}
		return err
	}
	return a.checkSpendConflictsOnce(tx)
}

func (a MempoolAdmission) checkSpendConflictsOnce(tx *wire.Tx) error {
	for i := range tx.Vin {
		in := &tx.Vin[i]
		if IsNullOutpoint(in) {
			continue
		}
		txid := txidDisplayFromLE(in.PrevHash)
		if a.Pool != nil && a.Pool.SpendsOutpoint(txid, in.PrevIdx) {
			return fmt.Errorf("%w (input %d)", ErrSpendInMempool, i)
		}
		if a.Chain != nil {
			spent, err := a.Chain.OutpointSpent(in.PrevHash, in.PrevIdx)
			if err != nil {
				return fmt.Errorf("input %d: %w", i, err)
			}
			if spent {
				return fmt.Errorf("%w (input %d)", ErrSpendOnChain, i)
			}
		}
	}
	return nil
}

func (a MempoolAdmission) standardPolicy() StandardPolicy {
	if a.Standard == (StandardPolicy{}) {
		return DefaultStandardPolicy()
	}
	pol := a.Standard
	if pol.HardDustLimitKoinu == 0 {
		pol.HardDustLimitKoinu = HardDustLimitKoinu
	}
	return pol
}

// CheckStandard applies Core IsStandardTx and AreInputsStandard mempool policy.
func (a MempoolAdmission) CheckStandard(tx *wire.Tx) error {
	pol := a.standardPolicy()
	if err := IsStandardTx(tx, pol, a.MinRelayFeePerKB); err != nil {
		return err
	}
	if a.View == nil {
		return nil
	}
	return AreInputsStandard(tx, a.View)
}
