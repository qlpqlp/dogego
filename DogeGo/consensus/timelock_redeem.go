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

// parseTimelockDropRedeem decodes <operand> OP_CHECKLOCKTIMEVERIFY|OP_CHECKSEQUENCEVERIFY OP_DROP <inner>.
func parseTimelockDropRedeem(script []byte, timeOpcode byte) (operand int64, tail []byte, err error) {
	if len(script) < 4 {
		return 0, nil, fmt.Errorf("timelock: script too short")
	}
	op, i, err := ReadScriptNumPush(script, 0)
	if err != nil {
		return 0, nil, err
	}
	if i >= len(script) || script[i] != timeOpcode {
		return 0, nil, fmt.Errorf("timelock: missing time verify opcode")
	}
	i++
	if i >= len(script) || script[i] != 0x75 {
		return 0, nil, fmt.Errorf("timelock: missing OP_DROP")
	}
	i++
	if i >= len(script) {
		return 0, nil, fmt.Errorf("timelock: missing inner script")
	}
	return op, script[i:], nil
}

func isTimelockRedeem(script []byte, timeOpcode byte) bool {
	_, tail, err := parseTimelockDropRedeem(script, timeOpcode)
	if err != nil {
		return false
	}
	return isP2PKHScript(tail) || isP2PKScript(tail) || IsMultisigRedeemScript(tail)
}

func verifyInputP2SHTimelock(tx *wire.Tx, idx int, redeem []byte, sigPushes [][]byte, flags ScriptVerifyFlags, timeOpcode byte) error {
	op, tail, err := parseTimelockDropRedeem(redeem, timeOpcode)
	if err != nil {
		return err
	}
	if timeOpcode == opCheckLockTimeVerify && flags&ScriptVerifyCheckLockTimeVerify != 0 {
		if err := CheckLockTime(tx, idx, op); err != nil {
			return fmt.Errorf("script-verify: %w", err)
		}
	}
	if timeOpcode == opCheckSequenceVerify && flags&ScriptVerifyCheckSequenceVerify != 0 {
		if err := CheckSequence(tx, idx, op); err != nil {
			return fmt.Errorf("script-verify: %w", err)
		}
	}
	switch {
	case isP2PKHScript(tail):
		if len(sigPushes) < 2 {
			return fmt.Errorf("script-verify: p2sh timelock P2PKH scriptSig too short")
		}
		innerSig := buildP2PKHScriptSig(sigPushes[0], sigPushes[1])
		return verifyInputP2PKHScriptSig(tx, idx, tail, innerSig, flags)
	case isP2PKScript(tail):
		if len(sigPushes) < 1 {
			return fmt.Errorf("script-verify: p2sh timelock P2PK scriptSig too short")
		}
		return verifyInputP2PKScriptSig(tx, idx, tail, buildSinglePushScript(sigPushes[0]), flags)
	case IsMultisigRedeemScript(tail):
		return verifyMultisigRedeem(tx, idx, tail, sigPushes, flags)
	default:
		return fmt.Errorf("script-verify: unsupported timelock inner redeem")
	}
}

// buildCLTVP2PKRedeemScript builds P2SH CLTV + bare P2PK for tests.
func buildCLTVP2PKRedeemScript(lockTime int64, pubKey []byte) []byte {
	var b []byte
	b = append(b, encodeScriptNum(lockTime)...)
	b = append(b, opCheckLockTimeVerify, 0x75)
	b = append(b, byte(len(pubKey)))
	b = append(b, pubKey...)
	b = append(b, 0xac)
	return b
}

// buildCSVP2PKRedeemScript builds P2SH CSV + bare P2PK for tests.
func buildCSVP2PKRedeemScript(relativeSequence int64, pubKey []byte) []byte {
	var b []byte
	b = append(b, encodeScriptNum(relativeSequence)...)
	b = append(b, opCheckSequenceVerify, 0x75)
	b = append(b, byte(len(pubKey)))
	b = append(b, pubKey...)
	b = append(b, 0xac)
	return b
}
