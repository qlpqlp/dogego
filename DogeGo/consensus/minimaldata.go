// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "fmt"

// ScriptVerifyMinimalData requires minimally encoded push operations (SCRIPT_VERIFY_MINIMALDATA).
const ScriptVerifyMinimalData ScriptVerifyFlags = 1 << 6

// checkMinimalPush reports whether opcode is the smallest way to push data (Core CheckMinimalPush).
func checkMinimalPush(data []byte, opcode byte) bool {
	if len(data) == 0 {
		return opcode == 0x00
	}
	if len(data) == 1 && data[0] >= 1 && data[0] <= 16 {
		return opcode == 0x50+data[0] // OP_1 (0x51) .. OP_16 (0x60)
	}
	if len(data) == 1 && data[0] == 0x81 {
		return opcode == 0x4f // OP_1NEGATE
	}
	if len(data) <= 75 {
		return opcode == byte(len(data))
	}
	if len(data) <= 255 {
		return opcode == 0x4c
	}
	if len(data) <= 65535 {
		return opcode == 0x4d
	}
	return true
}

// checkScriptMinimalData rejects non-minimal push encodings in push-only scripts (scriptSig / redeem pushes).
func checkScriptMinimalData(script []byte) error {
	i := 0
	for i < len(script) {
		if i >= len(script) {
			break
		}
		op := script[i]
		i++
		var data []byte
		opcode := op
		switch {
		case op == 0x00:
			data = []byte{}
		case op >= 0x01 && op <= 0x4b:
			n := int(op)
			if i+n > len(script) {
				return fmt.Errorf("script-verify: truncated push")
			}
			data = script[i : i+n]
			i += n
		case op == 0x4c:
			if i >= len(script) {
				return fmt.Errorf("script-verify: truncated pushdata1")
			}
			n := int(script[i])
			i++
			if i+n > len(script) {
				return fmt.Errorf("script-verify: truncated pushdata1 data")
			}
			data = script[i : i+n]
			i += n
		case op == 0x4d:
			if i+1 >= len(script) {
				return fmt.Errorf("script-verify: truncated pushdata2")
			}
			n := int(script[i]) | int(script[i+1])<<8
			i += 2
			if i+n > len(script) {
				return fmt.Errorf("script-verify: truncated pushdata2 data")
			}
			data = script[i : i+n]
			i += n
		case op == 0x4e:
			if i+3 >= len(script) {
				return fmt.Errorf("script-verify: truncated pushdata4")
			}
			n := int(script[i]) | int(script[i+1])<<8 | int(script[i+2])<<16 | int(script[i+3])<<24
			i += 4
			if i+n > len(script) {
				return fmt.Errorf("script-verify: truncated pushdata4 data")
			}
			data = script[i : i+n]
			i += n
		case op == 0x4f, op >= 0x51 && op <= 0x60:
			// OP_1NEGATE and OP_1..OP_16 are already minimally encoded.
			continue
		default:
			return fmt.Errorf("script-verify: non-push opcode 0x%02x", op)
		}
		if !checkMinimalPush(data, opcode) {
			return fmt.Errorf("script-verify: non-minimal push (MINIMALDATA)")
		}
	}
	return nil
}
