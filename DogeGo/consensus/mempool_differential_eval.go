// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"

	"dogego/chain"
	"dogego/mempool"
	"dogego/wire"
)

// EvaluateMempoolDifferentialCheck runs template-specific policy checks from core_mempool_vectors.json.
// Used by the differential harness and RPC integration tests for cases that need stub prevouts or RBF pools.
func EvaluateMempoolDifferentialCheck(template string) error {
	switch template {
	case "min_relay_fee":
		return evalMempoolMinRelayFee()
	case "rbf_insufficient_fee":
		return evalMempoolRBFInsufficientFee()
	case "rbf_sufficient_fee":
		return evalMempoolRBFSufficientFee()
	case "rbf_not_replaceable":
		return evalMempoolRBFNotReplaceable()
	case "rbf_fullrbf":
		return evalMempoolRBFFullRBFAccept()
	case "coinbase_immature":
		return evalMempoolCoinbaseImmature()
	case "rbf_too_many_descendants":
		return evalMempoolRBFTTooManyDescendants()
	case "rbf_too_many_conflicts":
		return evalMempoolRBFTooManyConflicts()
	case "rbf_new_unconfirmed_input":
		return evalMempoolRBFNewUnconfirmedInput()
	case "non_bip68_final":
		return evalMempoolNonBIP68Final()
	default:
		return fmt.Errorf("consensus: unknown mempool differential template %q", template)
	}
}

func evalMempoolMinRelayFee() error {
	var prev [32]byte
	prev[0] = 1
	view := mempoolStubPrevOutView{}
	view[outpointKey(prev, 0)] = PrevOut{Value: 200_000, PkScript: []byte{0x51}}
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 199_000, PkScript: []byte{0x51}}},
	}
	return CheckMinRelayFee(tx, view, DefaultMinRelayTxFeePerKB)
}

func evalMempoolRBFInsufficientFee() error {
	const parentVal = int64(200_000_000)
	parentID := txidDisplayFromLE([32]byte{9})
	view := fixedPrevOutView{
		rpcOutpointKey(parentID, 0): {Value: parentVal, PkScript: []byte{0x51}},
	}
	parentHash := [32]byte{9}
	old := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: []byte{0x51}}},
	}
	newTx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 199_950_000, PkScript: []byte{0x51}}},
	}
	oldRaw, _ := old.Serialize()
	oldID := txidDisplayFromLE(old.TxHash())
	pool := &rbfMockPool{
		spend: map[string]string{rpcOutpointKey(parentID, 0): oldID},
		raw:   map[string][]byte{oldID: oldRaw},
	}
	return TryResolveMempoolRBFConflicts(newTx, pool, view, false)
}

func evalMempoolRBFSufficientFee() error {
	raw, prep, view, err := BuildRBFSufficientFeeDifferentialFixture()
	if err != nil {
		return err
	}
	pool := mempool.New(100)
	if prep != nil {
		if err := prep(pool); err != nil {
			return err
		}
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return err
	}
	adm := MempoolAdmission{
		View:             view,
		Pool:             pool,
		RBFPool:          pool,
		MinRelayFeePerKB: DefaultMinRelayTxFeePerKB,
	}
	return AcceptMempoolTxPolicy(tx, adm)
}

func evalMempoolRBFNotReplaceable() error {
	raw, prep, view, err := BuildRBFNotReplaceableDifferentialFixture()
	if err != nil {
		return err
	}
	pool := mempool.New(100)
	if prep != nil {
		if err := prep(pool); err != nil {
			return err
		}
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return err
	}
	adm := MempoolAdmission{
		View:             view,
		Pool:             pool,
		RBFPool:          pool,
		MinRelayFeePerKB: DefaultMinRelayTxFeePerKB,
	}
	return AcceptMempoolTxPolicy(tx, adm)
}

func evalMempoolRBFFullRBFAccept() error {
	raw, prep, view, err := BuildRBFFullRBFAcceptFixture()
	if err != nil {
		return err
	}
	pool := mempool.New(100)
	if prep != nil {
		if err := prep(pool); err != nil {
			return err
		}
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return err
	}
	adm := MempoolAdmission{
		View:             view,
		Pool:             pool,
		RBFPool:          pool,
		MinRelayFeePerKB: DefaultMinRelayTxFeePerKB,
		FullRBF:          true,
	}
	return AcceptMempoolTxPolicy(tx, adm)
}

func evalMempoolCoinbaseImmature() error {
	spend, ix, j, _ := CoinbaseImmatureDifferentialSpend()
	adm := MempoolAdmission{
		Index:   ix,
		Journal: j,
		Net:     chain.RebootTestnet,
	}
	return adm.CheckCoinbaseMaturity(spend)
}

func evalMempoolRBFTTooManyDescendants() error {
	raw, prep, view, err := BuildRBFTTooManyDescendantsFixture()
	if err != nil {
		return err
	}
	pool := mempool.New(100)
	if prep != nil {
		if err := prep(pool); err != nil {
			return err
		}
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return err
	}
	adm := MempoolAdmission{
		View:             view,
		Pool:             pool,
		RBFPool:          pool,
		MinRelayFeePerKB: DefaultMinRelayTxFeePerKB,
	}
	return AcceptMempoolTxPolicy(tx, adm)
}

func evalMempoolRBFTooManyConflicts() error {
	raw, prep, view, err := BuildRBFTooManyConflictsFixture()
	if err != nil {
		return err
	}
	pool := mempool.New(200)
	if prep != nil {
		if err := prep(pool); err != nil {
			return err
		}
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return err
	}
	adm := MempoolAdmission{
		View:             view,
		Pool:             pool,
		RBFPool:          pool,
		MinRelayFeePerKB: DefaultMinRelayTxFeePerKB,
	}
	return AcceptMempoolTxPolicy(tx, adm)
}

func evalMempoolRBFNewUnconfirmedInput() error {
	raw, prep, view, err := BuildRBFNewUnconfirmedInputFixture()
	if err != nil {
		return err
	}
	pool := mempool.New(100)
	if prep != nil {
		if err := prep(pool); err != nil {
			return err
		}
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return err
	}
	adm := MempoolAdmission{
		View:             view,
		Pool:             pool,
		RBFPool:          pool,
		MinRelayFeePerKB: DefaultMinRelayTxFeePerKB,
	}
	return AcceptMempoolTxPolicy(tx, adm)
}

func evalMempoolNonBIP68Final() error {
	spend, j, _ := NonBIP68FinalDifferentialSpend()
	tip, err := j.TipHeight()
	if err != nil {
		return err
	}
	prevH := []int{499_995}
	return CheckTxSequenceLocks(spend, SequenceEvalBlock{Height: tip + 1}, prevH, j, chain.MainnetDogecoin)
}
