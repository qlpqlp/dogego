// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

func evalStackUnaryNum(stack [][]byte, flags ScriptVerifyFlags, fn func(int64) int64) ([][]byte, ScriptError) {
	if len(stack) < 1 {
		return stack, ScriptErrInvalidStackOperation
	}
	n, err := castStackInt(stack[len(stack)-1], flags)
	if err != ScriptErrOK {
		return stack, err
	}
	stack = stack[:len(stack)-1]
	stack = appendStack(stack, scriptNumPayload(fn(n)))
	return stack, ScriptErrOK
}

func evalStack0NotEqual(stack [][]byte, flags ScriptVerifyFlags) ([][]byte, ScriptError) {
	if len(stack) < 1 {
		return stack, ScriptErrInvalidStackOperation
	}
	n, err := castStackInt(stack[len(stack)-1], flags)
	if err != ScriptErrOK {
		return stack, err
	}
	if n == 0 {
		stack[len(stack)-1] = nil
	} else {
		stack[len(stack)-1] = []byte{1}
	}
	return stack, ScriptErrOK
}

func evalDisabledOpcode(stack [][]byte) ([][]byte, ScriptError) {
	return stack, ScriptErrDisabledOpcode
}
