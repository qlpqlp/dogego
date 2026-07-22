// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "fmt"

// ReadScriptNumPush reads a minimally-encoded script number push at off (CLTV/CSV locktimes, etc.).
func ReadScriptNumPush(script []byte, off int) (int64, int, error) {
	data, next, err := ReadScriptPush(script, off)
	if err != nil {
		return 0, off, fmt.Errorf("script-num: %w", err)
	}
	return DecodeScriptNum(data), next, nil
}

const scriptNumMaxBytes = 4

// decodeScriptNumArith decodes stack numbers for numeric ops (Core CScriptNum default 4-byte limit).
func decodeScriptNumArith(b []byte, flags ScriptVerifyFlags) (int64, ScriptError) {
	if flags&ScriptVerifyMinimalData != 0 {
		if !isMinimalScriptNum(b) {
			// Core maps non-minimal stack operands used as CScriptNum to UNKNOWN_ERROR (not MINIMALDATA).
			return 0, ScriptErrUnknown
		}
	}
	if len(b) > scriptNumMaxBytes {
		return 0, ScriptErrUnknown
	}
	n := DecodeScriptNum(b)
	if n < -2147483647 || n > 2147483647 {
		return 0, ScriptErrUnknown
	}
	return n, ScriptErrOK
}

// decodeScriptNumLocktime decodes stack numbers for CLTV/CSV (Core allows 5-byte operands).
func decodeScriptNumLocktime(b []byte, flags ScriptVerifyFlags) (int64, ScriptError) {
	if flags&ScriptVerifyMinimalData != 0 {
		if !isMinimalScriptNum(b) {
			return 0, ScriptErrUnknown
		}
	}
	if len(b) > 5 {
		return 0, ScriptErrUnknown
	}
	return DecodeScriptNum(b), ScriptErrOK
}

// scriptNumRawBytes returns the minimal signed-magnitude script-number encoding (Core CScriptNum).
func scriptNumRawBytes(n int64) []byte {
	if n == 0 {
		return nil
	}
	neg := n < 0
	abs := n
	if neg {
		abs = -n
	}
	var raw []byte
	for abs > 0 {
		raw = append(raw, byte(abs&0xff))
		abs >>= 8
	}
	if len(raw) == 0 {
		return nil
	}
	if raw[len(raw)-1]&0x80 != 0 {
		if neg {
			raw = append(raw, 0x80)
		} else {
			raw = append(raw, 0x00)
		}
	} else if neg {
		raw[len(raw)-1] |= 0x80
	}
	return raw
}

// DecodeScriptNum decodes a little-endian signed-magnitude script number (Core CScriptNum).
func DecodeScriptNum(b []byte) int64 {
	if len(b) == 0 {
		return 0
	}
	var n int64
	for i := 0; i < len(b); i++ {
		n |= int64(b[i]) << uint(8*i)
	}
	if b[len(b)-1]&0x80 != 0 {
		n &= ^(int64(0x80) << uint(8*(len(b)-1)))
		n = -n
	}
	return n
}
