// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "fmt"

// maxScriptElementSize is Core MAX_SCRIPT_ELEMENT_SIZE (interpreter.cpp).
const maxScriptElementSize = 520

// ReadScriptPush decodes the push at script[off] and returns data + next offset.
// Supports direct pushes, OP_PUSHDATA1/2/4, OP_1NEGATE, and OP_1..OP_16 (Core push encoding).
func ReadScriptPush(script []byte, off int) (data []byte, next int, err error) {
	if off >= len(script) {
		return nil, 0, fmt.Errorf("script-verify: truncated script")
	}
	op := script[off]
	off++
	switch {
	case op == 0x00:
		return []byte{}, off, nil
	case op >= 0x01 && op <= 0x4b:
		n := int(op)
		if off+n > len(script) {
			return nil, 0, fmt.Errorf("script-verify: truncated push")
		}
		return append([]byte(nil), script[off:off+n]...), off + n, nil
	case op == 0x4c:
		if off >= len(script) {
			return nil, 0, fmt.Errorf("script-verify: truncated pushdata1")
		}
		n := int(script[off])
		off++
		if off+n > len(script) {
			return nil, 0, fmt.Errorf("script-verify: truncated pushdata1 data")
		}
		return append([]byte(nil), script[off:off+n]...), off + n, nil
	case op == 0x4d:
		if off+1 >= len(script) {
			return nil, 0, fmt.Errorf("script-verify: truncated pushdata2")
		}
		n := int(script[off]) | int(script[off+1])<<8
		off += 2
		if off+n > len(script) {
			return nil, 0, fmt.Errorf("script-verify: truncated pushdata2 data")
		}
		return append([]byte(nil), script[off:off+n]...), off + n, nil
	case op == 0x4e:
		if off+3 >= len(script) {
			return nil, 0, fmt.Errorf("script-verify: truncated pushdata4")
		}
		n := int(script[off]) | int(script[off+1])<<8 | int(script[off+2])<<16 | int(script[off+3])<<24
		off += 4
		if off+n > len(script) {
			return nil, 0, fmt.Errorf("script-verify: truncated pushdata4 data")
		}
		return append([]byte(nil), script[off:off+n]...), off + n, nil
	case op == 0x4f:
		return []byte{0x81}, off, nil
	case op >= 0x51 && op <= 0x60:
		return []byte{byte(op - 0x50)}, off, nil
	default:
		return nil, 0, fmt.Errorf("script-verify: non-push opcode 0x%02x", op)
	}
}

// LastScriptPush returns the final push in a push-only script (P2SH scriptSig redeem script, etc.).
func LastScriptPush(script []byte) ([]byte, error) {
	var last []byte
	i := 0
	for i < len(script) {
		data, next, err := ReadScriptPush(script, i)
		if err != nil {
			return nil, err
		}
		last = data
		i = next
	}
	if last == nil && len(script) > 0 {
		return nil, fmt.Errorf("script-verify: empty scriptSig")
	}
	return last, nil
}
