// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"bytes"
	"encoding/hex"
	"strings"
	"unicode/utf8"
)

// Ord envelope tags (Bitcoin Ordinals / Doginals-compatible).
const (
	ordTagBody        = 0
	ordTagContentType = 1
)

// Envelope is a parsed OP_FALSE OP_IF … "ord" … OP_ENDIF inscription envelope.
type Envelope struct {
	ContentType string
	Body        []byte
}

// ParseOrdEnvelope extracts the first ord/doginal envelope from a tapscript-like byte stream
// (typically the last witness stack element). Compatible with standard Ordinals-style envelopes.
func ParseOrdEnvelope(script []byte) (Envelope, bool) {
	var z Envelope
	if len(script) < 6 {
		return z, false
	}
	i := 0
	for i < len(script) {
		// Find OP_FALSE (0x00) OP_IF (0x63)
		if script[i] != 0x00 {
			i++
			continue
		}
		if i+1 >= len(script) || script[i+1] != 0x63 {
			i++
			continue
		}
		i += 2
		push, next, ok := readPush(script, i)
		if !ok {
			return z, false
		}
		i = next
		if !bytes.Equal(push, []byte("ord")) && !bytes.EqualFold(push, []byte("doginal")) {
			// Not an inscription protocol marker; keep scanning.
			continue
		}
		var contentType string
		var body []byte
		for i < len(script) {
			if script[i] == 0x68 { // OP_ENDIF
				i++
				z.ContentType = contentType
				if z.ContentType == "" {
					z.ContentType = "application/octet-stream"
				}
				z.Body = body
				return z, len(body) > 0 || contentType != ""
			}
			// Bare tag integers 0..16 as single opcodes, or push of 1-byte tag.
			tag, nextTag, okTag := readOrdTag(script, i)
			if !okTag {
				return z, false
			}
			i = nextTag
			val, nextVal, okVal := readPush(script, i)
			if !okVal {
				return z, false
			}
			i = nextVal
			switch tag {
			case ordTagContentType:
				if utf8.Valid(val) {
					contentType = string(val)
				}
			case ordTagBody:
				body = append(body, val...)
			default:
				// Ignore unknown tags (parent, metadata, …).
			}
		}
		return z, false
	}
	return z, false
}

func readOrdTag(script []byte, i int) (int, int, bool) {
	if i >= len(script) {
		return 0, i, false
	}
	op := script[i]
	// OP_0 .. OP_16
	if op == 0x00 {
		return 0, i + 1, true
	}
	if op >= 0x51 && op <= 0x60 { // OP_1 .. OP_16
		return int(op - 0x50), i + 1, true
	}
	push, next, ok := readPush(script, i)
	if !ok || len(push) != 1 {
		return 0, i, false
	}
	return int(push[0]), next, true
}

func readPush(script []byte, i int) ([]byte, int, bool) {
	if i >= len(script) {
		return nil, i, false
	}
	op := script[i]
	i++
	var n int
	switch {
	case op == 0x00:
		return []byte{}, i, true
	case op <= 75:
		n = int(op)
	case op == 0x4c: // OP_PUSHDATA1
		if i >= len(script) {
			return nil, i, false
		}
		n = int(script[i])
		i++
	case op == 0x4d: // OP_PUSHDATA2
		if i+1 >= len(script) {
			return nil, i, false
		}
		n = int(script[i]) | int(script[i+1])<<8
		i += 2
	case op == 0x4e: // OP_PUSHDATA4
		if i+3 >= len(script) {
			return nil, i, false
		}
		n = int(script[i]) | int(script[i+1])<<8 | int(script[i+2])<<16 | int(script[i+3])<<24
		i += 4
	case op >= 0x51 && op <= 0x60: // OP_1..OP_16 as push of small int
		return []byte{op - 0x50}, i, true
	default:
		return nil, i - 1, false
	}
	if n < 0 || i+n > len(script) {
		return nil, i, false
	}
	return script[i : i+n], i + n, true
}

// DetectInscriptionFromWitness parses ord/doginal envelopes in an input witness.
func DetectInscriptionFromWitness(height int64, txid string, vin uint32, witness [][]byte) (Inscription, bool) {
	if len(witness) == 0 {
		return Inscription{}, false
	}
	// Ordinals put the envelope script in the last witness element (tapscript).
	script := witness[len(witness)-1]
	env, ok := ParseOrdEnvelope(script)
	if !ok {
		// Some doginal wallets put envelope in earlier stack items.
		for i := len(witness) - 2; i >= 0; i-- {
			env, ok = ParseOrdEnvelope(witness[i])
			if ok {
				break
			}
		}
	}
	if !ok {
		return Inscription{}, false
	}
	id := fmtInscriptionIDVin(height, txid, vin)
	ins := Inscription{
		ID:           id,
		Height:       height,
		TxID:         txid,
		Vout:         0,
		Vin:          vin,
		Kind:         "doginal",
		ContentType:  env.ContentType,
		PayloadHex:   hex.EncodeToString(env.Body),
		TextPreview:  previewText(env.Body, 96),
		Source:       "envelope",
	}
	if p, ok := ParseDRC20JSON(env.Body); ok {
		ins.Kind = "drc20"
		ins.Tick = p.Tick
		ins.Op = p.Op
		ins.Amount = firstNonEmpty(p.Amt, p.Max)
		ins.ContentType = "application/json"
		return ins, true
	}
	lowCT := strings.ToLower(env.ContentType)
	if strings.Contains(lowCT, "json") || (len(env.Body) > 0 && env.Body[0] == '{') {
		if p, ok := ParseDRC20JSON(env.Body); ok {
			ins.Kind = "drc20"
			ins.Tick = p.Tick
			ins.Op = p.Op
			ins.Amount = firstNonEmpty(p.Amt, p.Max)
		}
	}
	return ins, true
}

func fmtInscriptionIDVin(height int64, txid string, vin uint32) string {
	return strings.ToLower(txid) + "i" + itoa(int(vin)) + "@" + itoa64(height)
}
