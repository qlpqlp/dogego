// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// evalStackPick copies the n-th item from the top (n=0 is top) onto the stack (Core OP_PICK).
func evalStackPick(stack [][]byte, flags ScriptVerifyFlags) ([][]byte, ScriptError) {
	if len(stack) < 1 {
		return stack, ScriptErrInvalidStackOperation
	}
	n, serr := castStackInt(stack[len(stack)-1], flags)
	if serr != ScriptErrOK {
		return stack, serr
	}
	stack = stack[:len(stack)-1]
	if n < 0 || int(n) >= len(stack) {
		return stack, ScriptErrInvalidStackOperation
	}
	idx := len(stack) - 1 - int(n)
	stack = appendStack(stack, append([]byte(nil), stack[idx]...))
	return stack, ScriptErrOK
}

// evalStackRoll moves the n-th item from the top to the top (Core OP_ROLL).
func evalStackRoll(stack [][]byte, flags ScriptVerifyFlags) ([][]byte, ScriptError) {
	if len(stack) < 1 {
		return stack, ScriptErrInvalidStackOperation
	}
	n, serr := castStackInt(stack[len(stack)-1], flags)
	if serr != ScriptErrOK {
		return stack, serr
	}
	stack = stack[:len(stack)-1]
	if n < 0 || int(n) >= len(stack) {
		return stack, ScriptErrInvalidStackOperation
	}
	idx := len(stack) - 1 - int(n)
	item := stack[idx]
	stack = append(stack[:idx], stack[idx+1:]...)
	stack = appendStack(stack, item)
	return stack, ScriptErrOK
}

func evalStackRot(stack [][]byte) ([][]byte, ScriptError) {
	if len(stack) < 3 {
		return stack, ScriptErrInvalidStackOperation
	}
	n := len(stack)
	stack[n-3], stack[n-2], stack[n-1] = stack[n-2], stack[n-1], stack[n-3]
	return stack, ScriptErrOK
}

func evalStackOver(stack [][]byte) ([][]byte, ScriptError) {
	if len(stack) < 2 {
		return stack, ScriptErrInvalidStackOperation
	}
	stack = appendStack(stack, append([]byte(nil), stack[len(stack)-2]...))
	return stack, ScriptErrOK
}

func evalStackNip(stack [][]byte) ([][]byte, ScriptError) {
	if len(stack) < 2 {
		return stack, ScriptErrInvalidStackOperation
	}
	copy(stack[len(stack)-2:], stack[len(stack)-1:])
	stack = stack[:len(stack)-1]
	return stack, ScriptErrOK
}

func evalStackTuck(stack [][]byte) ([][]byte, ScriptError) {
	if len(stack) < 2 {
		return stack, ScriptErrInvalidStackOperation
	}
	top := append([]byte(nil), stack[len(stack)-1]...)
	idx := len(stack) - 2
	out := make([][]byte, 0, len(stack)+1)
	out = append(out, stack[:idx]...)
	out = append(out, top)
	out = append(out, stack[idx:]...)
	return out, ScriptErrOK
}

func evalStack2Dup(stack [][]byte) ([][]byte, ScriptError) {
	if len(stack) < 2 {
		return stack, ScriptErrInvalidStackOperation
	}
	a := append([]byte(nil), stack[len(stack)-2]...)
	b := append([]byte(nil), stack[len(stack)-1]...)
	stack = appendStack(stack, a)
	stack = appendStack(stack, b)
	return stack, ScriptErrOK
}

func evalStack3Dup(stack [][]byte) ([][]byte, ScriptError) {
	if len(stack) < 3 {
		return stack, ScriptErrInvalidStackOperation
	}
	a := append([]byte(nil), stack[len(stack)-3]...)
	b := append([]byte(nil), stack[len(stack)-2]...)
	c := append([]byte(nil), stack[len(stack)-1]...)
	stack = appendStack(stack, a)
	stack = appendStack(stack, b)
	stack = appendStack(stack, c)
	return stack, ScriptErrOK
}

func evalStack2Over(stack [][]byte) ([][]byte, ScriptError) {
	if len(stack) < 4 {
		return stack, ScriptErrInvalidStackOperation
	}
	stack = appendStack(stack, append([]byte(nil), stack[len(stack)-4]...))
	stack = appendStack(stack, append([]byte(nil), stack[len(stack)-4]...))
	return stack, ScriptErrOK
}

func evalStack2Rot(stack [][]byte) ([][]byte, ScriptError) {
	if len(stack) < 6 {
		return stack, ScriptErrInvalidStackOperation
	}
	n := len(stack)
	stack[n-6], stack[n-5], stack[n-4], stack[n-3], stack[n-2], stack[n-1] =
		stack[n-4], stack[n-3], stack[n-2], stack[n-1], stack[n-6], stack[n-5]
	return stack, ScriptErrOK
}

func evalStack2Swap(stack [][]byte) ([][]byte, ScriptError) {
	if len(stack) < 4 {
		return stack, ScriptErrInvalidStackOperation
	}
	n := len(stack)
	stack[n-4], stack[n-3], stack[n-2], stack[n-1] = stack[n-2], stack[n-1], stack[n-4], stack[n-3]
	return stack, ScriptErrOK
}

func castStackInt(b []byte, flags ScriptVerifyFlags) (int64, ScriptError) {
	return decodeScriptNumArith(b, flags)
}

// isMinimalScriptNum matches Core CScriptNum minimal encoding (no redundant leading zeros).
func isMinimalScriptNum(b []byte) bool {
	if len(b) == 0 {
		return true
	}
	if len(b) == 1 && (b[0] == 0x00 || b[0] == 0x80) {
		return false
	}
	if len(b) > 1 && ((b[len(b)-1]&0x7f) == 0) {
		if (b[len(b)-2]&0x80) == 0 {
			return false
		}
	}
	return true
}
