// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "dogego/wire"

// InnerSigningScriptFromP2SHRedeem returns the scriptCode used to sign a P2SH redeem (P2PKH inner for timelock PKH).
func InnerSigningScriptFromP2SHRedeem(redeem []byte) ([]byte, bool) {
	code, err := signingScriptCodeFromRedeemSimple(redeem)
	return code, err == nil
}

// CLTVLockTimeFromRedeem returns the absolute locktime for a P2SH redeem using OP_CHECKLOCKTIMEVERIFY.
func CLTVLockTimeFromRedeem(redeem []byte) (int64, bool) {
	if lock, _, err := parseTimelockMultisigRedeem(redeem, opCheckLockTimeVerify); err == nil {
		return lock, true
	}
	if lock, _, err := parseCLTVP2PKHRedeem(redeem); err == nil {
		return lock, true
	}
	if lock, _, err := parseTimelockDropRedeem(redeem, opCheckLockTimeVerify); err == nil {
		return lock, true
	}
	return 0, false
}

// CSVOperandFromRedeem returns the CSV sequence operand encoded in a P2SH redeem script.
func CSVOperandFromRedeem(redeem []byte) (int64, bool) {
	if seq, _, err := parseTimelockMultisigRedeem(redeem, opCheckSequenceVerify); err == nil {
		return seq, true
	}
	if seq, _, err := parseCSVP2PKHRedeem(redeem); err == nil {
		return seq, true
	}
	if seq, _, err := parseTimelockDropRedeem(redeem, opCheckSequenceVerify); err == nil {
		return seq, true
	}
	return 0, false
}

// CSVRequiredSequenceFromRedeem returns the minimum nSequence for spending a CSV P2SH redeem.
func CSVRequiredSequenceFromRedeem(redeem []byte) (uint32, bool) {
	op, ok := CSVOperandFromRedeem(redeem)
	if !ok {
		return 0, false
	}
	return CSVOperandToInputSequence(op), true
}

// CSVInputSequenceSatisfies reports whether inSeq meets the CSV operand (BIP112/BIP68 masks).
func CSVInputSequenceSatisfies(inSeq uint32, operand int64) bool {
	tx := &wire.Tx{Version: 2, Vin: []wire.TxIn{{Sequence: inSeq}}}
	return CheckSequence(tx, 0, operand) == nil
}

// CSVOperandToInputSequence maps a redeem-script CSV operand to a spending input nSequence.
func CSVOperandToInputSequence(operand int64) uint32 {
	if operand < 0 {
		return 0
	}
	mask := int64(wire.SequenceLocktimeTypeFlag | wire.SequenceLocktimeMask)
	opMasked := operand & mask
	if opMasked >= int64(wire.SequenceLocktimeTypeFlag) {
		if opMasked > 0xffffffff {
			return uint32(wire.SequenceLocktimeTypeFlag | wire.SequenceLocktimeMask)
		}
		return uint32(opMasked)
	}
	if operand > int64(wire.SequenceLocktimeMask) {
		return wire.SequenceLocktimeMask
	}
	return uint32(operand)
}
