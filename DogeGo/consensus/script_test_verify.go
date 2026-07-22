// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "dogego/wire"

// VerifyScriptTestSpend runs scriptSig then scriptPubKey with CHECKSIG wired to a synthetic spend tx.
func VerifyScriptTestSpend(scriptSig, scriptPubKey []byte, flags ScriptVerifyFlags) ScriptError {
	spend, _ := buildScriptTestCreditSpendFlags(scriptSig, scriptPubKey, flags)
	checker := &ScriptSpendChecker{Tx: spend, InputIdx: 0, Subscript: scriptPubKey}
	if flags&ScriptVerifyP2SH != 0 && isP2SHScript(scriptPubKey) {
		return verifyScriptTestP2SH(spend, scriptSig, scriptPubKey, flags, checker)
	}
	return verifyScriptTestBase(spend, scriptSig, scriptPubKey, flags, checker)
}

func verifyScriptTestBase(spend *wire.Tx, scriptSig, scriptPubKey []byte, flags ScriptVerifyFlags, checker ScriptSigChecker) ScriptError {
	if flags&ScriptVerifySigPushOnly != 0 && !isPushOnly(scriptSig) {
		return ScriptErrSigPushOnly
	}
	stack := [][]byte{}
	var err ScriptError
	stack, err = evalScript(stack, scriptSig, flags, checker)
	if err != ScriptErrOK {
		return err
	}
	stack, err = evalScript(stack, scriptPubKey, flags, checker)
	if err != ScriptErrOK {
		return err
	}
	return verifyScriptTestStackResult(stack, flags)
}

func verifyScriptTestStackResult(stack [][]byte, flags ScriptVerifyFlags) ScriptError {
	if flags&ScriptVerifyCleanStack != 0 && len(stack) != 1 {
		return ScriptErrCleanStack
	}
	if len(stack) == 0 || !scriptCastToBool(stack[len(stack)-1]) {
		return ScriptErrEvalFalse
	}
	return ScriptErrOK
}

func verifyScriptTestP2SH(spend *wire.Tx, scriptSig, scriptPubKey []byte, flags ScriptVerifyFlags, checker *ScriptSpendChecker) ScriptError {
	if !isPushOnly(scriptSig) {
		return ScriptErrSigPushOnly
	}
	stack := [][]byte{}
	var err ScriptError
	stack, err = evalScript(stack, scriptSig, flags, nil)
	if err != ScriptErrOK {
		return err
	}
	stackCopy := cloneStack(stack)
	stack, err = evalScript(stack, scriptPubKey, flags, nil)
	if err != ScriptErrOK {
		return err
	}
	if err := verifyScriptTestStackResult(stack, flags&^ScriptVerifyCleanStack); err != ScriptErrOK {
		return err
	}
	stack = stackCopy
	if len(stack) == 0 {
		return ScriptErrEvalFalse
	}
	redeem := stack[len(stack)-1]
	stack = stack[:len(stack)-1]
	checker.Subscript = redeem
	stack, err = evalScript(stack, redeem, flags, checker)
	if err != ScriptErrOK {
		return err
	}
	return verifyScriptTestStackResult(stack, flags)
}

func cloneStack(s [][]byte) [][]byte {
	out := make([][]byte, len(s))
	for i := range s {
		out[i] = append([]byte(nil), s[i]...)
	}
	return out
}
