// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"fmt"
)

// GetCFHeadersPayload is the BIP157 getcfheaders message body (same layout as getcfilters).
type GetCFHeadersPayload = FilterRangeRequest

// CFHeadersPayload is the BIP157 cfheaders message body.
type CFHeadersPayload struct {
	FilterType           byte
	StopHashLE           [32]byte
	PreviousFilterHeader [32]byte
	FilterHashes         [][32]byte
}

// EncodeCFHeadersPayload serializes cfheaders for P2P.
func EncodeCFHeadersPayload(p CFHeadersPayload) ([]byte, error) {
	n := len(p.FilterHashes)
	if n > MaxGetCFHeadersRange {
		return nil, fmt.Errorf("cfheaders: too many hashes %d", n)
	}
	var b bytes.Buffer
	if err := b.WriteByte(p.FilterType); err != nil {
		return nil, err
	}
	if _, err := b.Write(p.StopHashLE[:]); err != nil {
		return nil, err
	}
	if _, err := b.Write(p.PreviousFilterHeader[:]); err != nil {
		return nil, err
	}
	if err := WriteCompactSize(&b, uint64(n)); err != nil {
		return nil, err
	}
	for _, h := range p.FilterHashes {
		if len(h) != 32 {
			return nil, fmt.Errorf("cfheaders: hash must be 32 bytes")
		}
		if _, err := b.Write(h[:]); err != nil {
			return nil, err
		}
	}
	return b.Bytes(), nil
}

// DecodeCFHeadersPayload parses cfheaders (for tests).
func DecodeCFHeadersPayload(pl []byte) (CFHeadersPayload, error) {
	var out CFHeadersPayload
	if len(pl) < 65 {
		return out, fmt.Errorf("cfheaders: short payload")
	}
	r := bytes.NewReader(pl)
	ft, err := r.ReadByte()
	if err != nil {
		return out, err
	}
	out.FilterType = ft
	if _, err := r.Read(out.StopHashLE[:]); err != nil {
		return out, err
	}
	if _, err := r.Read(out.PreviousFilterHeader[:]); err != nil {
		return out, err
	}
	n, err := ReadCompactSize(r)
	if err != nil {
		return out, err
	}
	if n > MaxGetCFHeadersRange {
		return out, fmt.Errorf("cfheaders: too many hashes")
	}
	out.FilterHashes = make([][32]byte, n)
	for i := uint64(0); i < n; i++ {
		if _, err := r.Read(out.FilterHashes[i][:]); err != nil {
			return out, err
		}
	}
	if r.Len() != 0 {
		return out, fmt.Errorf("cfheaders: trailing bytes")
	}
	return out, nil
}
