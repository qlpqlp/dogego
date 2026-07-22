// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"math"
)

const scriptStackLimit = 1000
const maxOpsPerScript = 201
const maxScriptSize = 10000

// EvalScript executes script on stack (Core EvalScript subset for script_tests.json stack cases).
func EvalScript(stack [][]byte, script []byte, flags ScriptVerifyFlags) ([][]byte, ScriptError) {
	return evalScript(stack, script, flags, nil)
}

type scriptStackOverflow struct{}

func evalScript(stack [][]byte, script []byte, flags ScriptVerifyFlags, checker ScriptSigChecker) (stackOut [][]byte, serr ScriptError) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(scriptStackOverflow); ok {
				stackOut, serr = stack, ScriptErrStackSize
			} else {
				panic(r)
			}
		}
	}()
	if len(script) > maxScriptSize {
		return stack, ScriptErrScriptSize
	}
	if len(script) == 0 {
		return stack, ScriptErrOK
	}
	vfExec := []bool{true}
	alt := [][]byte{}
	nOpCount := 0
	i := 0
	for i < len(script) {
		exec := scriptExecActive(vfExec)
		op := script[i]
		if op > 0x00 && op <= 0x4e {
			data, next, err := ReadScriptPush(script, i)
			if err != nil {
				return stack, mapPushError(err)
			}
			if len(data) > maxScriptElementSize {
				return stack, ScriptErrPushSize
			}
			if !exec {
				i = next
				continue
			}
			if flags&ScriptVerifyMinimalData != 0 {
				if !checkMinimalPush(data, script[i]) {
					return stack, ScriptErrMinimalData
				}
			}
			stack = appendStack(stack, data)
			i = next
			continue
		}
		i++
		if opAlwaysDisabled(op) {
			return stack, ScriptErrDisabledOpcode
		}
		if opBadOpcodeInConditionalRange(op) {
			return stack, ScriptErrBadOpcode
		}
		if op > 0x60 && op != 0x50 {
			nOpCount++
			if nOpCount > maxOpsPerScript {
				return stack, ScriptErrOpCount
			}
		}
		switch op {
		case 0x00: // OP_0
			if exec {
				stack = appendStack(stack, nil)
			}
		case 0x4f: // OP_1NEGATE
			if exec {
				stack = appendStack(stack, []byte{0x81})
			}
		case 0x50: // OP_RESERVED
			if exec {
				return stack, ScriptErrBadOpcode
			}
		case 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5a, 0x5b, 0x5c, 0x5d, 0x5e, 0x5f, 0x60:
			if exec {
				stack = appendStack(stack, []byte{byte(op - 0x50)})
			}
		case 0x61: // OP_NOP
		case 0xb0, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7, 0xb8, 0xb9: // OP_NOP1, OP_NOP4..10
			if exec && flags&ScriptVerifyDiscourageUpgradableNops != 0 {
				return stack, ScriptErrDiscourageUpgradable
			}
		case opCheckLockTimeVerify:
			if !exec {
				continue
			}
			if checker == nil {
				checker = noopScriptChecker{}
			}
			var serr ScriptError
			stack, serr = evalCheckLockTimeVerify(stack, flags, checker)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case opCheckSequenceVerify:
			if !exec {
				continue
			}
			if checker == nil {
				checker = noopScriptChecker{}
			}
			var serr ScriptError
			stack, serr = evalCheckSequenceVerify(stack, flags, checker)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x62: // OP_VER
			if exec {
				return stack, ScriptErrBadOpcode
			}
		case 0x63: // OP_IF
			val := false
			if exec {
				if len(stack) < 1 {
					return stack, ScriptErrUnbalancedConditional
				}
				val = scriptCastToBool(stack[len(stack)-1])
				stack = stack[:len(stack)-1]
			}
			vfExec = append(vfExec, val)
		case 0x64: // OP_NOTIF
			val := false
			if exec {
				if len(stack) < 1 {
					return stack, ScriptErrUnbalancedConditional
				}
				val = !scriptCastToBool(stack[len(stack)-1])
				stack = stack[:len(stack)-1]
			}
			vfExec = append(vfExec, val)
		case 0x67: // OP_ELSE
			if len(vfExec) < 2 {
				return stack, ScriptErrUnbalancedConditional
			}
			vfExec[len(vfExec)-1] = !vfExec[len(vfExec)-1]
		case 0x68: // OP_ENDIF
			if len(vfExec) < 2 {
				return stack, ScriptErrUnbalancedConditional
			}
			vfExec = vfExec[:len(vfExec)-1]
		case 0x69: // OP_VERIFY
			if !exec {
				continue
			}
			if len(stack) < 1 {
				return stack, ScriptErrInvalidStackOperation
			}
			if !scriptCastToBool(stack[len(stack)-1]) {
				return stack, ScriptErrVerify
			}
			stack = stack[:len(stack)-1]
		case 0x6a: // OP_RETURN
			if exec {
				return stack, ScriptErrOpReturn
			}
		case 0x6b: // OP_TOALTSTACK
			if !exec {
				continue
			}
			if len(stack) < 1 {
				return stack, ScriptErrInvalidStackOperation
			}
			alt = append(alt, stack[len(stack)-1])
			stack = stack[:len(stack)-1]
		case 0x6c: // OP_FROMALTSTACK
			if !exec {
				continue
			}
			if len(alt) < 1 {
				return stack, ScriptErrInvalidAltStackOperation
			}
			stack = appendStack(stack, alt[len(alt)-1])
			alt = alt[:len(alt)-1]
		case 0x74: // OP_DEPTH
			if !exec {
				continue
			}
			depth := len(stack)
			if depth == 0 {
				stack = appendStack(stack, nil)
			} else if depth >= 1 && depth <= 16 {
				stack = appendStack(stack, []byte{byte(depth)})
			} else {
				stack = appendStack(stack, scriptNumPayload(int64(depth)))
			}
		case 0x75: // OP_DROP
			if !exec {
				continue
			}
			if len(stack) < 1 {
				return stack, ScriptErrInvalidStackOperation
			}
			stack = stack[:len(stack)-1]
		case 0x73: // OP_IFDUP
			if !exec {
				continue
			}
			if len(stack) < 1 {
				return stack, ScriptErrInvalidStackOperation
			}
			if scriptCastToBool(stack[len(stack)-1]) {
				stack = appendStack(stack, append([]byte(nil), stack[len(stack)-1]...))
			}
		case 0x76: // OP_DUP
			if !exec {
				continue
			}
			if len(stack) < 1 {
				return stack, ScriptErrInvalidStackOperation
			}
			stack = appendStack(stack, append([]byte(nil), stack[len(stack)-1]...))
		case 0x87: // OP_EQUAL
			if !exec {
				continue
			}
			if len(stack) < 2 {
				return stack, ScriptErrInvalidStackOperation
			}
			a := stack[len(stack)-2]
			b := stack[len(stack)-1]
			stack = stack[:len(stack)-2]
			if bytes.Equal(a, b) {
				stack = appendStack(stack, []byte{1})
			} else {
				stack = appendStack(stack, nil)
			}
		case 0x88: // OP_EQUALVERIFY
			if !exec {
				continue
			}
			if len(stack) < 2 {
				return stack, ScriptErrInvalidStackOperation
			}
			a := stack[len(stack)-2]
			b := stack[len(stack)-1]
			if !bytes.Equal(a, b) {
				return stack, ScriptErrEqualVerify
			}
			stack = stack[:len(stack)-2]
		case 0xa6, 0xa8, 0xaa: // OP_RIPEMD160, OP_SHA256, OP_HASH256
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalCryptoHash(stack, op)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0xa9: // OP_HASH160
			if !exec {
				continue
			}
			if len(stack) < 1 {
				return stack, ScriptErrInvalidStackOperation
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			h := hash160(top)
			stack = appendStack(stack, h[:])
		case 0x7c: // OP_SWAP
			if !exec {
				continue
			}
			if len(stack) < 2 {
				return stack, ScriptErrInvalidStackOperation
			}
			stack[len(stack)-2], stack[len(stack)-1] = stack[len(stack)-1], stack[len(stack)-2]
		case 0x82: // OP_SIZE
			if !exec {
				continue
			}
			if len(stack) < 1 {
				return stack, ScriptErrInvalidStackOperation
			}
			n := len(stack[len(stack)-1])
			stack = appendStack(stack, scriptNumPayload(int64(n)))
		case 0x6d: // OP_2DROP
			if !exec {
				continue
			}
			if len(stack) < 2 {
				return stack, ScriptErrInvalidStackOperation
			}
			stack = stack[:len(stack)-2]
		case 0x6e: // OP_2DUP
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStack2Dup(stack)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x6f: // OP_3DUP
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStack3Dup(stack)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x70: // OP_2OVER
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStack2Over(stack)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x71: // OP_2ROT
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStack2Rot(stack)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x72: // OP_2SWAP
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStack2Swap(stack)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x77: // OP_NIP
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStackNip(stack)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x78: // OP_OVER
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStackOver(stack)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x79: // OP_PICK
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStackPick(stack, flags)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x7a: // OP_ROLL
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStackRoll(stack, flags)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x7b: // OP_ROT
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStackRot(stack)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x7d: // OP_TUCK
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStackTuck(stack)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x9c: // OP_NUMEQUAL
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalNumEqual(stack, flags)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x9d: // OP_NUMEQUALVERIFY
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalNumEqualVerify(stack, flags)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x9e: // OP_NUMNOTEQUAL
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalNumCompare(stack, flags, func(a, b int64) bool { return a != b })
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x9f: // OP_LESSTHAN
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalNumCompare(stack, flags, func(a, b int64) bool { return a < b })
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0xa0: // OP_GREATERTHAN
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalNumCompare(stack, flags, func(a, b int64) bool { return a > b })
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0xa1: // OP_LESSTHANOREQUAL
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalNumCompare(stack, flags, func(a, b int64) bool { return a <= b })
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0xa2: // OP_GREATERTHANOREQUAL
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalNumCompare(stack, flags, func(a, b int64) bool { return a >= b })
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0xa3: // OP_MIN
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalNumMinMax(stack, flags, true)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0xa4: // OP_MAX
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalNumMinMax(stack, flags, false)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0xa5: // OP_WITHIN
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalNumWithin(stack, flags)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x94: // OP_SUB
			if !exec {
				continue
			}
			if len(stack) < 2 {
				return stack, ScriptErrInvalidStackOperation
			}
			a, serr := decodeScriptNumArith(stack[len(stack)-2], flags)
			if serr != ScriptErrOK {
				return stack, serr
			}
			b, serr := decodeScriptNumArith(stack[len(stack)-1], flags)
			if serr != ScriptErrOK {
				return stack, serr
			}
			stack = stack[:len(stack)-2]
			stack = appendStack(stack, scriptNumPayload(a-b))
		case 0xac: // OP_CHECKSIG
			if !exec {
				continue
			}
			if len(stack) < 2 {
				return stack, ScriptErrInvalidStackOperation
			}
			sig := stack[len(stack)-2]
			pub := stack[len(stack)-1]
			stack = stack[:len(stack)-2]
			if checker == nil {
				return stack, ScriptErrBadOpcode
			}
			ok, serr := checker.CheckSig(sig, pub, flags)
			if serr != ScriptErrOK {
				return stack, serr
			}
			if ok {
				stack = appendStack(stack, []byte{1})
			} else {
				stack = appendStack(stack, nil)
			}
		case 0xad: // OP_CHECKSIGVERIFY
			if !exec {
				continue
			}
			if len(stack) < 2 {
				return stack, ScriptErrInvalidStackOperation
			}
			sig := stack[len(stack)-2]
			pub := stack[len(stack)-1]
			stack = stack[:len(stack)-2]
			if checker == nil {
				return stack, ScriptErrBadOpcode
			}
			ok, serr := checker.CheckSig(sig, pub, flags)
			if serr != ScriptErrOK {
				return stack, serr
			}
			if !ok {
				return stack, ScriptErrCheckSigVerify
			}
		case 0x91: // OP_NOT
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStackUnaryNum(stack, flags, func(n int64) int64 {
				if n == 0 {
					return 1
				}
				return 0
			})
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x8b: // OP_1ADD
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStackUnaryNum(stack, flags, func(n int64) int64 { return n + 1 })
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x8c: // OP_1SUB
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStackUnaryNum(stack, flags, func(n int64) int64 { return n - 1 })
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x8f: // OP_NEGATE
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStackUnaryNum(stack, flags, func(n int64) int64 { return -n })
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x90: // OP_ABS
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStackUnaryNum(stack, flags, func(n int64) int64 {
				if n == math.MinInt64 {
					return n
				}
				if n < 0 {
					return -n
				}
				return n
			})
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x92: // OP_0NOTEQUAL
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalStack0NotEqual(stack, flags)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0xab: // OP_CODESEPARATOR
			if exec {
				setCheckerCodeSep(checker, i)
			}
		case 0x93: // OP_ADD
			if !exec {
				continue
			}
			if len(stack) < 2 {
				return stack, ScriptErrInvalidStackOperation
			}
			a, serr := decodeScriptNumArith(stack[len(stack)-2], flags)
			if serr != ScriptErrOK {
				return stack, serr
			}
			b, serr := decodeScriptNumArith(stack[len(stack)-1], flags)
			if serr != ScriptErrOK {
				return stack, serr
			}
			stack = stack[:len(stack)-2]
			stack = appendStack(stack, scriptNumPayload(a+b))
		case 0x9a: // OP_BOOLAND
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalNumCompare(stack, flags, func(a, b int64) bool { return a != 0 && b != 0 })
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0x9b: // OP_BOOLOR
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalNumCompare(stack, flags, func(a, b int64) bool { return a != 0 || b != 0 })
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0xae: // OP_CHECKMULTISIG
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalCheckMultiSig(stack, flags, checker, false, &nOpCount)
			if serr != ScriptErrOK {
				return stack, serr
			}
		case 0xaf: // OP_CHECKMULTISIGVERIFY
			if !exec {
				continue
			}
			var serr ScriptError
			stack, serr = evalCheckMultiSig(stack, flags, checker, true, &nOpCount)
			if serr != ScriptErrOK {
				return stack, serr
			}
		default:
			if exec {
				if opcodeDiscouraged(op, flags) {
					return stack, ScriptErrDiscourageUpgradable
				}
				return stack, ScriptErrBadOpcode
			}
		}
		if len(stack)+len(alt) > scriptStackLimit {
			return stack, ScriptErrStackSize
		}
	}
	if len(vfExec) != 1 {
		return stack, ScriptErrUnbalancedConditional
	}
	return stack, ScriptErrOK
}

// VerifyScriptTest runs scriptSig then scriptPubKey (Core script_tests harness).
func VerifyScriptTest(scriptSig, scriptPubKey []byte, flags ScriptVerifyFlags) ScriptError {
	spend, _ := buildScriptTestCreditSpendFlags(scriptSig, scriptPubKey, flags)
	checker := &ScriptSpendChecker{Tx: spend, InputIdx: 0, Subscript: scriptPubKey}
	return verifyScriptTestBase(spend, scriptSig, scriptPubKey, flags, checker)
}

func opAlwaysDisabled(op byte) bool {
	switch op {
	case 0x7e, 0x7f, 0x80, 0x81, 0x83, 0x84, 0x85, 0x86, 0x8d, 0x8e, 0x95, 0x96, 0x97, 0x98, 0x99:
		return true
	default:
		return false
	}
}

// opBadOpcodeInConditionalRange matches Core: opcodes between OP_IF and OP_ENDIF enter the switch even when inactive.
func opBadOpcodeInConditionalRange(op byte) bool {
	return op >= 0x63 && op <= 0x68 && op != 0x63 && op != 0x64 && op != 0x67 && op != 0x68
}

func appendStack(stack [][]byte, item []byte) [][]byte {
	if len(stack) >= scriptStackLimit {
		panic(scriptStackOverflow{})
	}
	return append(stack, item)
}

func scriptExecActive(vfExec []bool) bool {
	for _, v := range vfExec {
		if !v {
			return false
		}
	}
	return true
}

func scriptCastToBool(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	for i := 0; i < len(b)-1; i++ {
		if b[i] != 0 {
			return true
		}
	}
	if b[len(b)-1] == 0 {
		return false
	}
	if b[len(b)-1] == 0x80 {
		return false
	}
	return true
}

func mapPushError(err error) ScriptError {
	if err == nil {
		return ScriptErrOK
	}
	return ScriptErrBadOpcode
}
