// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// scriptCodeSepSetter is implemented by spend checkers that honor OP_CODESEPARATOR.
type scriptCodeSepSetter interface {
	setCodeSepOffset(off int)
}

func subscriptFromCodeSep(full []byte, begin int) []byte {
	if begin <= 0 {
		return full
	}
	if begin >= len(full) {
		return nil
	}
	return full[begin:]
}

func setCheckerCodeSep(checker ScriptSigChecker, off int) {
	if c, ok := checker.(scriptCodeSepSetter); ok {
		c.setCodeSepOffset(off)
	}
}
