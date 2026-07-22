// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

func evalNumCompare(stack [][]byte, flags ScriptVerifyFlags, cmp func(a, b int64) bool) ([][]byte, ScriptError) {
	if len(stack) < 2 {
		return stack, ScriptErrInvalidStackOperation
	}
	a, err := castStackInt(stack[len(stack)-2], flags)
	if err != ScriptErrOK {
		return stack, err
	}
	b, err := castStackInt(stack[len(stack)-1], flags)
	if err != ScriptErrOK {
		return stack, err
	}
	stack = stack[:len(stack)-2]
	if cmp(a, b) {
		stack = appendStack(stack, []byte{1})
	} else {
		stack = appendStack(stack, nil)
	}
	return stack, ScriptErrOK
}

func evalNumEqual(stack [][]byte, flags ScriptVerifyFlags) ([][]byte, ScriptError) {
	return evalNumCompare(stack, flags, func(a, b int64) bool { return a == b })
}

func evalNumEqualVerify(stack [][]byte, flags ScriptVerifyFlags) ([][]byte, ScriptError) {
	if len(stack) < 2 {
		return stack, ScriptErrInvalidStackOperation
	}
	a, err := castStackInt(stack[len(stack)-2], flags)
	if err != ScriptErrOK {
		return stack, err
	}
	b, err := castStackInt(stack[len(stack)-1], flags)
	if err != ScriptErrOK {
		return stack, err
	}
	if a != b {
		return stack, ScriptErrNumEqualVerify
	}
	return stack[:len(stack)-2], ScriptErrOK
}

func evalNumWithin(stack [][]byte, flags ScriptVerifyFlags) ([][]byte, ScriptError) {
	if len(stack) < 3 {
		return stack, ScriptErrInvalidStackOperation
	}
	x, err := castStackInt(stack[len(stack)-3], flags)
	if err != ScriptErrOK {
		return stack, err
	}
	lo, err := castStackInt(stack[len(stack)-2], flags)
	if err != ScriptErrOK {
		return stack, err
	}
	hi, err := castStackInt(stack[len(stack)-1], flags)
	if err != ScriptErrOK {
		return stack, err
	}
	stack = stack[:len(stack)-3]
	if lo <= x && x < hi {
		stack = appendStack(stack, []byte{1})
	} else {
		stack = appendStack(stack, nil)
	}
	return stack, ScriptErrOK
}

func evalNumMinMax(stack [][]byte, flags ScriptVerifyFlags, pickMin bool) ([][]byte, ScriptError) {
	if len(stack) < 2 {
		return stack, ScriptErrInvalidStackOperation
	}
	a, err := castStackInt(stack[len(stack)-2], flags)
	if err != ScriptErrOK {
		return stack, err
	}
	b, err := castStackInt(stack[len(stack)-1], flags)
	if err != ScriptErrOK {
		return stack, err
	}
	stack = stack[:len(stack)-2]
	var n int64
	if pickMin {
		if a < b {
			n = a
		} else {
			n = b
		}
	} else {
		if a > b {
			n = a
		} else {
			n = b
		}
	}
	stack = appendStack(stack, scriptNumPayload(n))
	return stack, ScriptErrOK
}
