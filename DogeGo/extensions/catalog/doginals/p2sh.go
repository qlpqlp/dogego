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

// MaxInscriptionBodyBytes caps stored media bodies (P2SH / envelope).
const MaxInscriptionBodyBytes = 4 << 20 // 4 MiB

// scriptChunk is one Bitcoin script push or opcode (apezord doginals style).
type scriptChunk struct {
	Op  byte
	Buf []byte
}

// ParseScriptChunks splits a script into pushes / small-number opcodes.
func ParseScriptChunks(script []byte) []scriptChunk {
	var out []scriptChunk
	i := 0
	for i < len(script) {
		op := script[i]
		i++
		switch {
		case op == 0x00: // OP_0
			out = append(out, scriptChunk{Op: op})
		case op >= 0x01 && op <= 0x4b: // direct push
			n := int(op)
			if i+n > len(script) {
				return out
			}
			out = append(out, scriptChunk{Op: op, Buf: append([]byte(nil), script[i:i+n]...)})
			i += n
		case op == 0x4c: // OP_PUSHDATA1
			if i >= len(script) {
				return out
			}
			n := int(script[i])
			i++
			if i+n > len(script) {
				return out
			}
			out = append(out, scriptChunk{Op: op, Buf: append([]byte(nil), script[i:i+n]...)})
			i += n
		case op == 0x4d: // OP_PUSHDATA2
			if i+1 >= len(script) {
				return out
			}
			n := int(script[i]) | int(script[i+1])<<8
			i += 2
			if i+n > len(script) {
				return out
			}
			out = append(out, scriptChunk{Op: op, Buf: append([]byte(nil), script[i:i+n]...)})
			i += n
		case op == 0x4e: // OP_PUSHDATA4
			if i+3 >= len(script) {
				return out
			}
			n := int(script[i]) | int(script[i+1])<<8 | int(script[i+2])<<16 | int(script[i+3])<<24
			i += 4
			if n < 0 || i+n > len(script) {
				return out
			}
			out = append(out, scriptChunk{Op: op, Buf: append([]byte(nil), script[i:i+n]...)})
			i += n
		case op >= 0x51 && op <= 0x60: // OP_1..OP_16
			out = append(out, scriptChunk{Op: op})
		default:
			out = append(out, scriptChunk{Op: op})
		}
	}
	return out
}

// chunkToNumber mirrors apezord doginals.js chunkToNumber.
func chunkToNumber(c scriptChunk) (int, bool) {
	switch {
	case c.Op == 0x00:
		return 0, true
	case c.Op == 0x01 && len(c.Buf) == 1:
		return int(c.Buf[0]), true
	case c.Op == 0x02 && len(c.Buf) == 2:
		return int(c.Buf[0]) + int(c.Buf[1])*256, true
	case c.Op >= 0x51 && c.Op <= 0x60:
		return int(c.Op - 0x50), true
	default:
		return 0, false
	}
}

// p2shPartial is inscription pushdata extracted from one P2SH unlock scriptSig.
type p2shPartial struct {
	StartsOrd   bool
	Pieces      int // only set when StartsOrd
	ContentType string
	Parts       [][]byte // data chunks in order encountered (may be empty if only header)
	// ExpectedNext is the next number separator expected (pieces-1 after header, then descending).
	// After consuming a part with separator n, ExpectedNext becomes n-1 conceptually via Remaining.
	Separators []int
	RawChunks  int
}

// ExtractP2SHInscriptionPartial parses apezord/booktoshi P2SH scriptSig unlock data.
// Unlock layout: inscription pushdatas + signature + redeemScript (last two pushes ignored when present).
// Protocol: https://github.com/apezord/doginals  -  redeem scripts start with inscription push datas.
func ExtractP2SHInscriptionPartial(scriptSig []byte) (p2shPartial, bool) {
	var z p2shPartial
	chunks := ParseScriptChunks(scriptSig)
	if len(chunks) == 0 {
		return z, false
	}
	// Drop trailing signature + redeemScript when unlock is long enough (apezord mint path).
	dataChunks := chunks
	if len(chunks) >= 3 {
		last := chunks[len(chunks)-1]
		prev := chunks[len(chunks)-2]
		// Redeem scripts are typically larger than a sighash; signatures are 65-73 bytes-ish.
		if len(last.Buf) >= 25 && len(prev.Buf) >= 8 && len(prev.Buf) <= 80 {
			dataChunks = chunks[:len(chunks)-2]
		}
	}
	if len(dataChunks) == 0 {
		return z, false
	}
	z.RawChunks = len(dataChunks)

	i := 0
	if len(dataChunks[0].Buf) > 0 && (bytes.Equal(dataChunks[0].Buf, []byte("ord")) || bytes.EqualFold(dataChunks[0].Buf, []byte("doginal"))) {
		z.StartsOrd = true
		i = 1
		if i >= len(dataChunks) {
			return z, false
		}
		pieces, ok := chunkToNumber(dataChunks[i])
		if !ok || pieces < 0 {
			return z, false
		}
		z.Pieces = pieces
		i++
		if i >= len(dataChunks) {
			return z, false
		}
		if len(dataChunks[i].Buf) == 0 && dataChunks[i].Op != 0x00 {
			return z, false
		}
		z.ContentType = string(dataChunks[i].Buf)
		if !utf8.ValidString(z.ContentType) {
			z.ContentType = "application/octet-stream"
		}
		i++
	}

	for i < len(dataChunks) {
		n, ok := chunkToNumber(dataChunks[i])
		if !ok {
			break
		}
		i++
		if i >= len(dataChunks) {
			break
		}
		part := dataChunks[i].Buf
		i++
		z.Separators = append(z.Separators, n)
		z.Parts = append(z.Parts, append([]byte(nil), part...))
	}
	if z.StartsOrd {
		return z, true
	}
	// Continuation txs have only number+data pairs.
	return z, len(z.Parts) > 0
}

// p2shAssembly tracks multi-tx doginal content while separators count down to 0.
type p2shAssembly struct {
	Pieces      int
	Remaining   int
	ContentType string
	Data        []byte
	StartTxID   string
	StartHeight int64
	Vin         uint32
}

func (a *p2shAssembly) applyPartial(p p2shPartial) bool {
	if a == nil {
		return false
	}
	if p.StartsOrd {
		a.Pieces = p.Pieces
		a.Remaining = p.Pieces
		a.ContentType = p.ContentType
		a.Data = a.Data[:0]
	}
	for i, n := range p.Separators {
		if a.Remaining <= 0 {
			break
		}
		if n != a.Remaining-1 {
			// Out of order / not our continuation.
			return false
		}
		if i < len(p.Parts) {
			a.Data = append(a.Data, p.Parts[i]...)
		}
		a.Remaining--
	}
	return true
}

func (a *p2shAssembly) complete() bool {
	return a != nil && a.Pieces > 0 && a.Remaining == 0 && (len(a.Data) > 0 || a.ContentType != "")
}

// ClassifyMediaKind labels indexed content for UI (token | image | text | json | file).
func ClassifyMediaKind(contentType string, body []byte, isDRC20 bool) string {
	if isDRC20 {
		return "token"
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	switch {
	case strings.HasPrefix(ct, "image/"):
		return "image"
	case strings.HasPrefix(ct, "video/"), strings.HasPrefix(ct, "audio/"):
		return "file"
	case ct == "application/json" || (len(body) > 0 && body[0] == '{'):
		return "json"
	case strings.HasPrefix(ct, "text/"):
		return "text"
	case sniffIsImage(body):
		return "image"
	default:
		if ct == "" || ct == "application/octet-stream" {
			if utf8.Valid(body) && looksMostlyText(body) {
				return "text"
			}
		}
		return "file"
	}
}

func sniffIsImage(b []byte) bool {
	if len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff {
		return true
	}
	if len(b) >= 8 && string(b[:8]) == "\x89PNG\r\n\x1a\n" {
		return true
	}
	if len(b) >= 6 && (string(b[:6]) == "GIF87a" || string(b[:6]) == "GIF89a") {
		return true
	}
	if len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP" {
		return true
	}
	return false
}

func looksMostlyText(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	n := len(b)
	if n > 512 {
		n = 512
	}
	ctrl := 0
	for _, c := range b[:n] {
		if c < 0x09 || (c > 0x0d && c < 0x20) {
			ctrl++
		}
	}
	return ctrl*10 < n
}

// InscriptionFromBody builds a classified inscription from reconstructed content.
func InscriptionFromBody(height int64, txid string, vin uint32, contentType string, body []byte, source string) Inscription {
	if len(body) > MaxInscriptionBodyBytes {
		body = body[:MaxInscriptionBodyBytes]
	}
	id := fmtInscriptionIDVin(height, txid, vin)
	ins := Inscription{
		ID:          id,
		Height:      height,
		TxID:        txid,
		Vin:         vin,
		Kind:        "doginal",
		ContentType: contentType,
		TextPreview: previewText(body, 96),
		Source:      source,
		Size:        len(body),
		HasContent:  len(body) > 0,
	}
	if contentType == "" {
		ins.ContentType = sniffContentType(body)
	}
	if p, ok := ParseDRC20JSON(body); ok {
		ins.Kind = "drc20"
		ins.Tick = p.Tick
		ins.Op = p.Op
		ins.Amount = firstNonEmpty(p.Amt, p.Max)
		ins.ContentType = "application/json"
		ins.MediaKind = "token"
		ins.Body = append([]byte(nil), body...)
		if len(body) <= 2048 {
			ins.PayloadHex = hex.EncodeToString(body)
		}
		return ins
	}
	ins.MediaKind = ClassifyMediaKind(ins.ContentType, body, false)
	ins.Body = append([]byte(nil), body...)
	if ins.MediaKind == "json" || ins.MediaKind == "text" {
		if len(body) <= 2048 {
			ins.PayloadHex = hex.EncodeToString(body)
		}
	} else if len(body) <= 512 {
		ins.PayloadHex = hex.EncodeToString(body)
	}
	return ins
}
