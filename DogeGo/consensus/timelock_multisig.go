// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "fmt"

// parseTimelockMultisigRedeem decodes <operand> OP_CHECKLOCKTIMEVERIFY|OP_CHECKSEQUENCEVERIFY OP_DROP <multisig>.
func parseTimelockMultisigRedeem(script []byte, timeOpcode byte) (operand int64, multisig []byte, err error) {
	op, tail, err := parseTimelockDropRedeem(script, timeOpcode)
	if err != nil {
		return 0, nil, err
	}
	if !IsMultisigRedeemScript(tail) {
		return 0, nil, fmt.Errorf("timelock-multisig: tail is not multisig")
	}
	return op, tail, nil
}

func isCLTVMultisigRedeem(script []byte) bool {
	_, _, err := parseTimelockMultisigRedeem(script, opCheckLockTimeVerify)
	return err == nil
}

func isCSVMultisigRedeem(script []byte) bool {
	_, _, err := parseTimelockMultisigRedeem(script, opCheckSequenceVerify)
	return err == nil
}

// BuildCLTVMultisigRedeemScript builds P2SH CLTV + bare multisig redeem (tests / RPC tooling).
func BuildCLTVMultisigRedeemScript(lockTime int64, multisig []byte) []byte {
	return buildCLTVMultisigRedeemScript(lockTime, multisig)
}

func buildCLTVMultisigRedeemScript(lockTime int64, multisig []byte) []byte {
	var b []byte
	b = append(b, encodeScriptNum(lockTime)...)
	b = append(b, opCheckLockTimeVerify, 0x75)
	b = append(b, multisig...)
	return b
}

// BuildCSVMultisigRedeemScript builds P2SH CSV + bare multisig redeem (tests / RPC tooling).
func BuildCSVMultisigRedeemScript(relativeSequence int64, multisig []byte) []byte {
	return buildCSVMultisigRedeemScript(relativeSequence, multisig)
}

func buildCSVMultisigRedeemScript(relativeSequence int64, multisig []byte) []byte {
	var b []byte
	b = append(b, encodeScriptNum(relativeSequence)...)
	b = append(b, opCheckSequenceVerify, 0x75)
	b = append(b, multisig...)
	return b
}
