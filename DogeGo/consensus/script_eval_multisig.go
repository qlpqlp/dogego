// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

const maxPubkeysPerMultisigEval = 20

func evalCheckMultiSig(stack [][]byte, flags ScriptVerifyFlags, checker ScriptSigChecker, verify bool, nOpCount *int) ([][]byte, ScriptError) {
	i := 1
	if len(stack) < i {
		return stack, ScriptErrInvalidStackOperation
	}
	nKeys64, serr := castStackInt(stack[len(stack)-i], flags)
	if serr != ScriptErrOK {
		return stack, serr
	}
	nKeys := int(nKeys64)
	if nKeys < 0 || nKeys > maxPubkeysPerMultisigEval {
		return stack, ScriptErrPubKeyCount
	}
	if nOpCount != nil {
		*nOpCount += nKeys
		if *nOpCount > maxOpsPerScript {
			return stack, ScriptErrOpCount
		}
	}
	i++
	ikey := i
	i += nKeys
	if len(stack) < i {
		return stack, ScriptErrInvalidStackOperation
	}
	nSigs, serr := castStackInt(stack[len(stack)-i], flags)
	if serr != ScriptErrOK {
		return stack, serr
	}
	nSigsCount := int(nSigs)
	if nSigs < 0 || nSigsCount > nKeys {
		return stack, ScriptErrSigCount
	}
	isig := i + 1
	i = isig + int(nSigs)
	if len(stack) < i {
		return stack, ScriptErrInvalidStackOperation
	}
	if checker == nil {
		return stack, ScriptErrBadOpcode
	}
	ok := true
	keysLeft := nKeys
	sigsLeft := int(nSigs)
	for ok && sigsLeft > 0 {
		sig := stack[len(stack)-isig]
		pub := stack[len(stack)-ikey]
		if err := checkPubKeyEncoding(pub, flags); err != ScriptErrOK {
			return stack, err
		}
		if err := checkSignatureEncoding(sig, flags); err != nil {
			return stack, scriptErrFromSigEncoding(err)
		}
		valid, serr := checker.CheckSig(sig, pub, flags)
		if serr != ScriptErrOK {
			return stack, serr
		}
		if valid {
			isig++
			sigsLeft--
		}
		ikey++
		keysLeft--
		if sigsLeft > keysLeft {
			ok = false
		}
	}
	// Core removes i-1 items, then the extra dummy push, then pushes the bool result.
	ikey2 := nKeys + 2
	for i > 1 {
		i--
		if !ok && flags&ScriptVerifyNullFail != 0 && ikey2 == 0 {
			if len(stack[len(stack)-1]) != 0 {
				return stack, ScriptErrSigNullFail
			}
		}
		if ikey2 > 0 {
			ikey2--
		}
		if len(stack) < 1 {
			return stack, ScriptErrInvalidStackOperation
		}
		stack = stack[:len(stack)-1]
	}
	if len(stack) < 1 {
		return stack, ScriptErrInvalidStackOperation
	}
	if flags&ScriptVerifyNullDummy != 0 && len(stack[len(stack)-1]) != 0 {
		return stack, ScriptErrSigNullDummy
	}
	stack = stack[:len(stack)-1]
	if ok {
		stack = appendStack(stack, []byte{1})
	} else {
		stack = appendStack(stack, nil)
	}
	if verify {
		if !ok {
			return stack, ScriptErrCheckSigVerify
		}
		stack = stack[:len(stack)-1]
	}
	return stack, ScriptErrOK
}
