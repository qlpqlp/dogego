// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"fmt"
	"io"
)

// GetCFCheckptPayload is the BIP157 getcfcheckpt message body.
type GetCFCheckptPayload struct {
	FilterType byte
	StopHashLE [32]byte
}

// DecodeGetCFCheckptPayload parses getcfcheckpt (filter_type, stop_hash).
func DecodeGetCFCheckptPayload(pl []byte) (GetCFCheckptPayload, error) {
	var out GetCFCheckptPayload
	if len(pl) < 33 {
		return out, fmt.Errorf("getcfcheckpt: short payload")
	}
	out.FilterType = pl[0]
	r := bytes.NewReader(pl[1:])
	if _, err := io.ReadFull(r, out.StopHashLE[:]); err != nil {
		return out, err
	}
	if r.Len() != 0 {
		return out, fmt.Errorf("getcfcheckpt: trailing bytes")
	}
	return out, nil
}

// CFCheckptPayload is the BIP157 cfcheckpt message body.
type CFCheckptPayload struct {
	FilterType    byte
	StopHashLE    [32]byte
	FilterHeaders [][32]byte
}

// EncodeCFCheckptPayload serializes cfcheckpt for P2P.
func EncodeCFCheckptPayload(p CFCheckptPayload) ([]byte, error) {
	var b bytes.Buffer
	if err := b.WriteByte(p.FilterType); err != nil {
		return nil, err
	}
	if _, err := b.Write(p.StopHashLE[:]); err != nil {
		return nil, err
	}
	n := len(p.FilterHeaders)
	if err := WriteCompactSize(&b, uint64(n)); err != nil {
		return nil, err
	}
	for _, h := range p.FilterHeaders {
		if len(h) != 32 {
			return nil, fmt.Errorf("cfcheckpt: header must be 32 bytes")
		}
		if _, err := b.Write(h[:]); err != nil {
			return nil, err
		}
	}
	return b.Bytes(), nil
}
