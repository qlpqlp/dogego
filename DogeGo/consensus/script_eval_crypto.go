// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"crypto/sha256"

	"golang.org/x/crypto/ripemd160"
)

func evalCryptoHash(stack [][]byte, op byte) ([][]byte, ScriptError) {
	if len(stack) < 1 {
		return stack, ScriptErrInvalidStackOperation
	}
	top := stack[len(stack)-1]
	stack = stack[:len(stack)-1]
	var out []byte
	switch op {
	case 0xa6: // OP_RIPEMD160
		r := ripemd160.New()
		_, _ = r.Write(top)
		out = r.Sum(nil)
	case 0xa8: // OP_SHA256
		s := sha256.Sum256(top)
		out = s[:]
	case 0xaa: // OP_HASH256
		s := sha256.Sum256(top)
		s2 := sha256.Sum256(s[:])
		out = s2[:]
	default:
		return stack, ScriptErrBadOpcode
	}
	return appendStack(stack, out), ScriptErrOK
}
