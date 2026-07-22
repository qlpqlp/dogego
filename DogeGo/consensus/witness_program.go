// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// ParseWitnessProgram reports whether script is a BIP141 witness program (OP_0 or OP_1..16 + 2..40 byte push).
func ParseWitnessProgram(script []byte) (version int, ok bool) {
	if len(script) < 4 || len(script) > 42 {
		return 0, false
	}
	op := script[0]
	if op != 0x00 && (op < 0x51 || op > 0x60) {
		return 0, false
	}
	if int(script[1])+2 != len(script) {
		return 0, false
	}
	if op == 0x00 {
		return 0, true
	}
	return int(op - 0x50), true
}

// IsWitnessScriptPubKey is true for v0/v1+ witness output templates (BIP141).
func IsWitnessScriptPubKey(script []byte) bool {
	_, ok := ParseWitnessProgram(script)
	return ok
}
