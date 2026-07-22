// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"dogego/wire"
)

var scriptOpcodeByName map[string]byte

func init() {
	scriptOpcodeByName = make(map[string]byte)
	for op := 0; op <= 0xff; op++ {
		name := wire.OpcodeName(byte(op))
		if name == "" || strings.HasPrefix(name, "OP_UNKNOWN") {
			continue
		}
		scriptOpcodeByName[name] = byte(op)
		if strings.HasPrefix(name, "OP_") {
			scriptOpcodeByName[strings.TrimPrefix(name, "OP_")] = byte(op)
		}
		if op >= 0x51 && op <= 0x60 {
			scriptOpcodeByName[strconv.Itoa(int(op - 0x50))] = byte(op)
		}
	}
}

// ParseScriptASM compiles Core decodescript/asm text (script_tests.json format) to bytecode.
func ParseScriptASM(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var out []byte
	for _, w := range strings.Fields(s) {
		if w == "" {
			continue
		}
		if isScriptAsmDecimal(w) {
			n, err := strconv.ParseInt(w, 10, 64)
			if err != nil {
				return nil, err
			}
			out = appendScriptInt64(out, n)
			continue
		}
		if strings.HasPrefix(w, "0x") && len(w) > 2 {
			raw, err := hex.DecodeString(w[2:])
			if err != nil {
				return nil, fmt.Errorf("script asm hex: %w", err)
			}
			out = append(out, raw...)
			continue
		}
		if len(w) >= 2 && w[0] == '\'' && w[len(w)-1] == '\'' {
			out = appendScriptBytes(out, []byte(w[1:len(w)-1]))
			continue
		}
		op, ok := scriptOpcodeByName[w]
		if !ok {
			return nil, fmt.Errorf("script asm: unknown token %q", w)
		}
		out = append(out, op)
	}
	return out, nil
}

func isScriptAsmDecimal(w string) bool {
	if w == "" {
		return false
	}
	if w[0] == '-' {
		w = w[1:]
	}
	if w == "" {
		return false
	}
	for _, c := range w {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func appendScriptBytes(script []byte, data []byte) []byte {
	if len(data) == 0 {
		return append(script, 0x00)
	}
	if len(data) == 1 && data[0] >= 1 && data[0] <= 16 {
		return append(script, 0x50+data[0])
	}
	if len(data) <= 75 {
		script = append(script, byte(len(data)))
		return append(script, data...)
	}
	if len(data) <= 0xff {
		script = append(script, 0x4c, byte(len(data)))
		return append(script, data...)
	}
	if len(data) <= 0xffff {
		script = append(script, 0x4d, byte(len(data)), byte(len(data)>>8))
		return append(script, data...)
	}
	script = append(script, 0x4e,
		byte(len(data)), byte(len(data)>>8), byte(len(data)>>16), byte(len(data)>>24))
	return append(script, data...)
}

func appendScriptInt64(script []byte, n int64) []byte {
	if n == 0 {
		return append(script, 0x00)
	}
	if n == -1 {
		return append(script, 0x4f)
	}
	if n >= 1 && n <= 16 {
		return append(script, byte(0x50+n))
	}
	return appendScriptBytes(script, scriptNumPayload(n))
}

// scriptNumPayload is the raw little-endian script-number bytes (stack element), without push opcode.
func scriptNumPayload(n int64) []byte {
	return scriptNumRawBytes(n)
}

// ScriptToASM disassembles script bytecode to Core decodescript-style asm text.
func ScriptToASM(script []byte) string {
	var parts []string
	for i := 0; i < len(script); {
		op := script[i]
		if op >= 0x51 && op <= 0x60 {
			parts = append(parts, wire.OpcodeName(op))
			i++
			continue
		}
		if op == 0x00 {
			parts = append(parts, "0")
			i++
			continue
		}
		if op >= 1 && op <= 75 {
			n := int(op)
			if i+1+n > len(script) {
				parts = append(parts, hex.EncodeToString(script[i:]))
				break
			}
			parts = append(parts, hex.EncodeToString(script[i+1:i+1+n]))
			i += 1 + n
			continue
		}
		if op == 0x4c {
			if i+1 >= len(script) {
				parts = append(parts, "OP_PUSHDATA1", hex.EncodeToString(script[i+1:]))
				break
			}
			n := int(script[i+1])
			if i+2+n > len(script) {
				parts = append(parts, "OP_PUSHDATA1", hex.EncodeToString(script[i+1:]))
				break
			}
			parts = append(parts, hex.EncodeToString(script[i+2:i+2+n]))
			i += 2 + n
			continue
		}
		if op == 0x4d {
			if i+2 >= len(script) {
				parts = append(parts, "OP_PUSHDATA2", hex.EncodeToString(script[i+1:]))
				break
			}
			n := int(script[i+1]) | int(script[i+2])<<8
			if i+3+n > len(script) {
				parts = append(parts, "OP_PUSHDATA2", hex.EncodeToString(script[i+1:]))
				break
			}
			parts = append(parts, hex.EncodeToString(script[i+3:i+3+n]))
			i += 3 + n
			continue
		}
		if op == 0x4e {
			if i+4 >= len(script) {
				parts = append(parts, "OP_PUSHDATA4", hex.EncodeToString(script[i+1:]))
				break
			}
			n := int(script[i+1]) | int(script[i+2])<<8 | int(script[i+3])<<16 | int(script[i+4])<<24
			if i+5+n > len(script) {
				parts = append(parts, "OP_PUSHDATA4", hex.EncodeToString(script[i+1:]))
				break
			}
			parts = append(parts, hex.EncodeToString(script[i+5:i+5+n]))
			i += 5 + n
			continue
		}
		parts = append(parts, scriptASMOpcodeName(op))
		i++
	}
	return strings.Join(parts, " ")
}

func scriptASMOpcodeName(op byte) string {
	name := wire.OpcodeName(op)
	if strings.HasPrefix(name, "OP_") {
		short := strings.TrimPrefix(name, "OP_")
		if _, ok := scriptOpcodeByName[short]; ok {
			return short
		}
	}
	if name == "" {
		return "OP_UNKNOWN_" + strings.ToUpper(hex.EncodeToString([]byte{op}))
	}
	return name
}
