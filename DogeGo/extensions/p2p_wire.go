// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// DogeGo extension negotiation uses new P2P commands after Dogecoin verack.
// Unknown commands are ignored by Core nodes (no consensus impact).

const (
	CmdExtHello = "exthello"
	CmdExtAck   = "extack"
)

// EncodeExtHello builds exthello payload: varstr-list of supported protocol ids.
func EncodeExtHello(supported []string) []byte {
	return encodeStringList(supported)
}

// DecodeExtHello parses exthello.
func DecodeExtHello(payload []byte) ([]string, error) {
	return decodeStringList(payload)
}

// EncodeExtAck builds extack payload: varstr-list of enabled protocol ids.
func EncodeExtAck(enabled []string) []byte {
	return encodeStringList(enabled)
}

// DecodeExtAck parses extack.
func DecodeExtAck(payload []byte) ([]string, error) {
	return decodeStringList(payload)
}

func encodeStringList(items []string) []byte {
	var out []byte
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(len(items)))
	out = append(out, n[:]...)
	for _, s := range items {
		b := []byte(strings.TrimSpace(s))
		if len(b) > 0xffff {
			b = b[:0xffff]
		}
		binary.LittleEndian.PutUint16(n[:2], uint16(len(b)))
		out = append(out, n[:2]...)
		out = append(out, b...)
	}
	return out
}

func decodeStringList(payload []byte) ([]string, error) {
	if len(payload) < 4 {
		return nil, fmt.Errorf("extension list too short")
	}
	n := binary.LittleEndian.Uint32(payload[:4])
	off := 4
	var out []string
	for i := uint32(0); i < n; i++ {
		if off+2 > len(payload) {
			return nil, fmt.Errorf("truncated extension list")
		}
		ln := int(binary.LittleEndian.Uint16(payload[off : off+2]))
		off += 2
		if off+ln > len(payload) {
			return nil, fmt.Errorf("truncated extension string")
		}
		out = append(out, string(payload[off:off+ln]))
		off += ln
	}
	return out, nil
}
