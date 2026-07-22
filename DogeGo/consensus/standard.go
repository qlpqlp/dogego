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

// ErrNonStandardTx is returned when a transaction fails relay standardness policy.
var ErrNonStandardTx = errors.New("consensus: non-standard transaction")

// Core policy defaults (policy.h / policy.cpp).
const (
	MaxStandardTxVersion   = 2
	MaxStandardTxWeight    = 400_000
	MaxStandardScriptSig   = 1650
	MaxP2SHSigOps          = 15
	MaxDatacarrierBytes           = 83
	HardDustLimitKoinu            = 100_000 // DEFAULT_HARD_DUST_LIMIT = RECOMMENDED_MIN_TX_FEE/10 (0.001 DOGE)
	RecommendedMinTxFee           = 1_000_000
	MinStandardTxNonWitnessSize   = 82 // policy.h MIN_STANDARD_TX_NONWITNESS_SIZE
)

// StandardPolicy configures mempool standardness (Core node policy).
type StandardPolicy struct {
	HardDustLimitKoinu  int64
	AllowBareMultisig   bool
	AcceptDataCarrier   bool
	MaxDatacarrierBytes int // 0 uses MaxDatacarrierBytes default
}

// DefaultStandardPolicy returns Dogecoin Core defaults.
func DefaultStandardPolicy() StandardPolicy {
	return StandardPolicy{
		HardDustLimitKoinu: HardDustLimitKoinu,
		AllowBareMultisig:  true,
		AcceptDataCarrier:  true,
	}
}

// ScriptTemplate classifies standard scriptPubKey templates.
type ScriptTemplate int

const (
	ScriptNonStandard ScriptTemplate = iota
	ScriptPubKeyHash
	ScriptScriptHash
	ScriptMultisig
	ScriptNullData
	ScriptWitnessProgram
)

// ClassifyScriptPubKey reports the standard template for scriptPubKey (witness disabled).
func ClassifyScriptPubKey(script []byte) ScriptTemplate {
	if IsWitnessScriptPubKey(script) {
		return ScriptWitnessProgram
	}
	if isP2PKHScript(script) {
		return ScriptPubKeyHash
	}
	if isP2SHScript(script) {
		return ScriptScriptHash
	}
	if isNullDataScript(script) {
		return ScriptNullData
	}
	if nReq, _, err := ParseMultisigRedeemScript(script); err == nil {
		nKeys := countMultisigKeys(script)
		if nKeys >= 1 && nKeys <= 3 && nReq >= 1 && nReq <= nKeys {
			return ScriptMultisig
		}
	}
	return ScriptNonStandard
}

func countMultisigKeys(script []byte) int {
	if len(script) < 3 || script[len(script)-1] != 0xae {
		return 0
	}
	i := 1
	n := 0
	for i < len(script)-2 {
		if i >= len(script) {
			break
		}
		ln := int(script[i])
		i++
		if ln != 33 && ln != 65 {
			break
		}
		if i+ln > len(script) {
			break
		}
		n++
		i += ln
	}
	return n
}

func isNullDataScript(script []byte) bool {
	return len(script) >= 1 && script[0] == 0x6a
}

// IsStandardScript reports whether scriptPubKey is a standard output type (Core IsStandard; witness disabled on Dogecoin).
func IsStandardScript(script []byte, pol StandardPolicy) bool {
	typ := ClassifyScriptPubKey(script)
	if typ == ScriptWitnessProgram {
		return false
	}
	switch typ {
	case ScriptPubKeyHash, ScriptScriptHash, ScriptMultisig:
		return true
	case ScriptNullData:
		if !pol.AcceptDataCarrier {
			return false
		}
		maxCarrier := pol.MaxDatacarrierBytes
		if maxCarrier <= 0 {
			maxCarrier = MaxDatacarrierBytes
		}
		return len(script) <= maxCarrier
	default:
		if IsUnspendableScript(script) {
			return false
		}
		return false
	}
}

// TransactionWeight returns BIP141 weight (witness discount when stacks present).
func TransactionWeight(tx *wire.Tx) (int, error) {
	return wire.TransactionWeight(tx)
}

// IsOutputDust reports value below the hard dust relay limit (non-OP_RETURN outputs only).
func IsOutputDust(out wire.TxOut, limit int64) bool {
	if isNullDataScript(out.PkScript) {
		return false
	}
	return out.Value < limit
}

// IsUnspendableScript reports provably unspendable scriptPubKeys (Core CScript::IsUnspendable).
func IsUnspendableScript(script []byte) bool {
	if len(script) == 0 {
		return false
	}
	if script[0] == 0x6a {
		return true
	}
	return len(script) > MaxBlockBaseSize
}

// IsStandardTx applies Core IsStandardTx policy (mempool relay only).
// dustRelayPerKB is used for fee-sized dust (0 = DefaultMinRelayTxFeePerKB).
func IsStandardTx(tx *wire.Tx, pol StandardPolicy, dustRelayPerKB uint64) error {
	if tx == nil {
		return fmt.Errorf("consensus: nil transaction")
	}
	if tx.Version > MaxStandardTxVersion || tx.Version < 1 {
		return fmt.Errorf("%w: version", ErrNonStandardTx)
	}
	wt, err := TransactionWeight(tx)
	if err != nil {
		return err
	}
	if wt >= MaxStandardTxWeight {
		return fmt.Errorf("%w: tx-size", ErrNonStandardTx)
	}
	raw, err := tx.Serialize()
	if err != nil {
		return err
	}
	if len(raw) < MinStandardTxNonWitnessSize {
		return fmt.Errorf("%w: tx-size-small", ErrNonStandardTx)
	}
	for i := range tx.Vin {
		if len(tx.Vin[i].Script) > MaxStandardScriptSig {
			return fmt.Errorf("%w: scriptsig-size", ErrNonStandardTx)
		}
		if !isPushOnly(tx.Vin[i].Script) {
			return fmt.Errorf("%w: scriptsig-not-pushonly", ErrNonStandardTx)
		}
	}
	var dataOuts int
	for _, out := range tx.Vout {
		if !IsStandardScript(out.PkScript, pol) {
			return fmt.Errorf("%w: scriptpubkey", ErrNonStandardTx)
		}
		typ := ClassifyScriptPubKey(out.PkScript)
		if typ == ScriptNullData {
			if out.Value != 0 {
				return fmt.Errorf("%w: non-zero-op-return", ErrNonStandardTx)
			}
			dataOuts++
		} else if typ == ScriptMultisig && !pol.AllowBareMultisig {
			return fmt.Errorf("%w: bare-multisig", ErrNonStandardTx)
		} else if IsOutputDustEffective(out, pol, dustRelayPerKB) {
			return fmt.Errorf("%w: dust", ErrNonStandardTx)
		}
	}
	if dataOuts > 1 {
		return fmt.Errorf("%w: multi-op-return", ErrNonStandardTx)
	}
	return nil
}

// AreInputsStandard rejects expensive P2SH redeem scripts (Core AreInputsStandard).
func AreInputsStandard(tx *wire.Tx, view PrevOutView) error {
	if tx == nil || IsCoinbaseTx(tx) {
		return nil
	}
	if view == nil {
		return nil
	}
	pol := DefaultStandardPolicy()
	for i := range tx.Vin {
		prev, ok := view.Lookup(tx.Vin[i].PrevHash, tx.Vin[i].PrevIdx)
		if !ok {
			continue
		}
		if !IsStandardScript(prev.PkScript, pol) {
			return fmt.Errorf("%w: non-standard-input", ErrNonStandardTx)
		}
		if ClassifyScriptPubKey(prev.PkScript) != ScriptScriptHash {
			continue
		}
		redeem, err := lastScriptPush(tx.Vin[i].Script)
		if err != nil || len(redeem) == 0 {
			return fmt.Errorf("%w: p2sh-redeem", ErrNonStandardTx)
		}
		if CountSigOps(redeem, true) > MaxP2SHSigOps {
			return fmt.Errorf("%w: p2sh-sigops", ErrNonStandardTx)
		}
	}
	return nil
}
