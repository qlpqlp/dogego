// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "dogego/wire"

func evalCheckLockTimeVerify(stack [][]byte, flags ScriptVerifyFlags, checker ScriptSigChecker) ([][]byte, ScriptError) {
	if flags&ScriptVerifyCheckLockTimeVerify == 0 {
		if flags&ScriptVerifyDiscourageUpgradableNops != 0 {
			return stack, ScriptErrDiscourageUpgradable
		}
		return stack, ScriptErrOK
	}
	if len(stack) < 1 {
		return stack, ScriptErrInvalidStackOperation
	}
	lockTime, serr := decodeScriptNumLocktime(stack[len(stack)-1], flags)
	if serr != ScriptErrOK {
		return stack, serr
	}
	if lockTime < 0 {
		return stack, ScriptErrNegativeLocktime
	}
	tx, idx := checker.SpendContext()
	if tx == nil {
		return stack, ScriptErrBadOpcode
	}
	if err := CheckLockTime(tx, idx, lockTime); err != nil {
		return stack, ScriptErrUnsatisfiedLocktime
	}
	return stack, ScriptErrOK
}

func evalCheckSequenceVerify(stack [][]byte, flags ScriptVerifyFlags, checker ScriptSigChecker) ([][]byte, ScriptError) {
	if flags&ScriptVerifyCheckSequenceVerify == 0 {
		if flags&ScriptVerifyDiscourageUpgradableNops != 0 {
			return stack, ScriptErrDiscourageUpgradable
		}
		return stack, ScriptErrOK
	}
	if len(stack) < 1 {
		return stack, ScriptErrInvalidStackOperation
	}
	operand, serr := decodeScriptNumLocktime(stack[len(stack)-1], flags)
	if serr != ScriptErrOK {
		return stack, serr
	}
	if operand < 0 {
		return stack, ScriptErrNegativeLocktime
	}
	if operand&int64(wire.SequenceLocktimeDisableFlag) != 0 {
		return stack, ScriptErrOK
	}
	tx, idx := checker.SpendContext()
	if tx == nil {
		return stack, ScriptErrBadOpcode
	}
	if err := CheckSequence(tx, idx, operand); err != nil {
		return stack, ScriptErrUnsatisfiedLocktime
	}
	return stack, ScriptErrOK
}
