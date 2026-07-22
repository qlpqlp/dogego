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

// MaxP2SHRedeemDepth limits nested P2SH-forward redeems (Core practical bound).
const MaxP2SHRedeemDepth = 3

// isP2SHForwardRedeem reports HASH160 <20> OP_EQUAL (P2SH output script used as nested redeem).
func isP2SHForwardRedeem(script []byte) bool {
	return isP2SHScript(script)
}

func verifyP2SHRedeemScript(tx *wire.Tx, idx int, redeem []byte, sigPushes [][]byte, flags ScriptVerifyFlags) error {
	return verifyP2SHRedeemScriptDepth(tx, idx, redeem, sigPushes, flags, 0)
}

func verifyP2SHRedeemScriptDepth(tx *wire.Tx, idx int, redeem []byte, sigPushes [][]byte, flags ScriptVerifyFlags, depth int) error {
	if depth > MaxP2SHRedeemDepth {
		return fmt.Errorf("script-verify: p2sh redeem recursion limit")
	}
	switch {
	case isP2PKHScript(redeem):
		if len(sigPushes) < 2 {
			return fmt.Errorf("script-verify: p2sh P2PKH scriptSig too short")
		}
		innerSig := buildP2PKHScriptSig(sigPushes[0], sigPushes[1])
		return verifyInputP2PKHScriptSig(tx, idx, redeem, innerSig, flags)
	case IsMultisigRedeemScript(redeem):
		return verifyMultisigRedeem(tx, idx, redeem, sigPushes, flags)
	case isP2PKScript(redeem):
		if len(sigPushes) < 1 {
			return fmt.Errorf("script-verify: p2sh P2PK scriptSig too short")
		}
		return verifyInputP2PKScriptSig(tx, idx, redeem, buildSinglePushScript(sigPushes[0]), flags)
	case isTimelockRedeem(redeem, opCheckLockTimeVerify):
		return verifyInputP2SHTimelock(tx, idx, redeem, sigPushes, flags, opCheckLockTimeVerify)
	case isTimelockRedeem(redeem, opCheckSequenceVerify):
		return verifyInputP2SHTimelock(tx, idx, redeem, sigPushes, flags, opCheckSequenceVerify)
	case isP2SHForwardRedeem(redeem):
		if len(sigPushes) < 1 {
			return fmt.Errorf("script-verify: nested p2sh scriptSig too short")
		}
		innerRedeem := sigPushes[len(sigPushes)-1]
		var wantH [20]byte
		copy(wantH[:], redeem[2:22])
		if hash160(innerRedeem) != wantH {
			return fmt.Errorf("script-verify: nested p2sh hash mismatch")
		}
		return verifyP2SHRedeemScriptDepth(tx, idx, innerRedeem, sigPushes[:len(sigPushes)-1], flags, depth+1)
	default:
		return fmt.Errorf("script-verify: unsupported p2sh redeem script")
	}
}
