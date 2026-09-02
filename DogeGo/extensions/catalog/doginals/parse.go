// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"encoding/hex"
	"strings"
	"unicode/utf8"

	"dogego/wire"
)

// ExtractOPReturnPayload returns data after OP_RETURN (0x6a), or nil.
func ExtractOPReturnPayload(pkScript []byte) []byte {
	if len(pkScript) < 2 || pkScript[0] != 0x6a {
		return nil
	}
	i := 1
	if i >= len(pkScript) {
		return nil
	}
	op := pkScript[i]
	i++
	var n int
	switch {
	case op <= 75:
		n = int(op)
	case op == 0x4c && i < len(pkScript): // OP_PUSHDATA1
		n = int(pkScript[i])
		i++
	case op == 0x4d && i+1 < len(pkScript): // OP_PUSHDATA2
		n = int(pkScript[i]) | int(pkScript[i+1])<<8
		i += 2
	default:
		return nil
	}
	if n <= 0 || i+n > len(pkScript) {
		return nil
	}
	return pkScript[i : i+n]
}

// DetectInscriptionFromOutput classifies a tx output for L1 indexing.
func DetectInscriptionFromOutput(height int64, txid string, vout uint32, o wire.TxOut) (Inscription, bool) {
	payload := ExtractOPReturnPayload(o.PkScript)
	if len(payload) == 0 {
		return Inscription{}, false
	}
	id := fmtInscriptionID(height, txid, vout)
	ins := Inscription{
		ID:           id,
		Height:       height,
		TxID:         txid,
		Vout:         vout,
		PayloadHex:   hex.EncodeToString(payload),
		TextPreview:  previewText(payload, 96),
	}
	if p, ok := ParseDRC20JSON(payload); ok {
		ins.Kind = "drc20"
		ins.Tick = p.Tick
		ins.Op = p.Op
		ins.Amount = firstNonEmpty(p.Amt, p.Max)
		ins.ContentType = "application/json"
		ins.Source = "opreturn"
		return ins, true
	}
	low := strings.ToLower(string(payload))
	if strings.Contains(low, "doginal") || strings.HasPrefix(low, "ord") || strings.Contains(low, "text/plain") || strings.Contains(low, "image/") {
		ins.Kind = "doginal"
		ins.ContentType = sniffContentType(payload)
		ins.Source = "opreturn"
		return ins, true
	}
	ins.Kind = "data"
	ins.ContentType = sniffContentType(payload)
	ins.Source = "opreturn"
	return ins, true
}

func fmtInscriptionID(height int64, txid string, vout uint32) string {
	return strings.ToLower(txid) + "i" + itoa(int(vout)) + "@" + itoa64(height)
}

func previewText(b []byte, max int) string {
	if !utf8.Valid(b) {
		return ""
	}
	s := string(b)
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func sniffContentType(b []byte) string {
	if len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff {
		return "image/jpeg"
	}
	if len(b) >= 8 && string(b[:8]) == "\x89PNG\r\n\x1a\n" {
		return "image/png"
	}
	if len(b) > 0 && b[0] == '{' {
		return "application/json"
	}
	if utf8.Valid(b) {
		return "text/plain"
	}
	return "application/octet-stream"
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func itoa64(n int64) string { return itoa(int(n)) }
