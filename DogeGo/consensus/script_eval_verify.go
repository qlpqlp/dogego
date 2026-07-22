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

// txSpendChecker implements ScriptSigChecker for production VerifyScript (Core BaseSignatureChecker).
type txSpendChecker struct {
	tx           *wire.Tx
	inputIdx     int
	subscript    []byte
	codeSepBegin int
}

func (c *txSpendChecker) setCodeSepOffset(off int) {
	if c != nil {
		c.codeSepBegin = off
	}
}

func (c *txSpendChecker) SpendContext() (*wire.Tx, int) {
	return c.tx, c.inputIdx
}

func (c *txSpendChecker) CheckSig(sig, pubKey []byte, flags ScriptVerifyFlags) (bool, ScriptError) {
	sub := subscriptFromCodeSep(c.subscript, c.codeSepBegin)
	code := scriptCodeForSignature(sub, sig)
	return evalCheckSig(c.tx, c.inputIdx, code, sig, pubKey, flags)
}

// scriptCodeForSignature applies Core CScript::FindAndDelete (raw byte match, not push-aware).
func scriptCodeForSignature(subscript, sig []byte) []byte {
	if len(sig) == 0 {
		return subscript
	}
	return findAndDeleteScriptBytes(subscript, sig)
}

func findAndDeleteScriptBytes(script, pattern []byte) []byte {
	if len(pattern) == 0 || len(script) == 0 {
		return append([]byte(nil), script...)
	}
	out := make([]byte, 0, len(script))
	for i := 0; i < len(script); {
		if i+len(pattern) <= len(script) && bytesEqual(script[i:i+len(pattern)], pattern) {
			i += len(pattern)
			continue
		}
		out = append(out, script[i])
		i++
	}
	return out
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// verifyInputEval runs scriptSig and scriptPubKey via EvalScript (Core VerifyScript BASE path).
func verifyInputEval(tx *wire.Tx, idx int, pkScript []byte, flags ScriptVerifyFlags) error {
	scriptSig := tx.Vin[idx].Script
	checker := &txSpendChecker{tx: tx, inputIdx: idx, subscript: pkScript}
	var serr ScriptError
	if isP2SHScript(pkScript) {
		serr = verifyInputEvalP2SH(scriptSig, pkScript, flags, checker)
	} else {
		serr = verifyInputEvalBase(scriptSig, pkScript, flags, checker)
	}
	return scriptErrorToGo(serr)
}

func verifyInputEvalBase(scriptSig, scriptPubKey []byte, flags ScriptVerifyFlags, checker ScriptSigChecker) ScriptError {
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
	if len(stack) == 0 || !scriptCastToBool(stack[len(stack)-1]) {
		return ScriptErrEvalFalse
	}
	return ScriptErrOK
}

func verifyInputEvalP2SH(scriptSig, scriptPubKey []byte, flags ScriptVerifyFlags, checker *txSpendChecker) ScriptError {
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
	if len(stack) == 0 || !scriptCastToBool(stack[len(stack)-1]) {
		return ScriptErrEvalFalse
	}
	stack = stackCopy
	if len(stack) == 0 {
		return ScriptErrEvalFalse
	}
	redeem := stack[len(stack)-1]
	stack = stack[:len(stack)-1]
	checker.subscript = redeem
	stack, err = evalScript(stack, redeem, flags, checker)
	if err != ScriptErrOK {
		return err
	}
	if len(stack) == 0 || !scriptCastToBool(stack[len(stack)-1]) {
		return ScriptErrEvalFalse
	}
	return ScriptErrOK
}

func scriptErrorToGo(err ScriptError) error {
	if err == ScriptErrOK {
		return nil
	}
	return fmt.Errorf("script-verify: %s", err)
}
