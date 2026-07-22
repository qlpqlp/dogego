// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"encoding/binary"
)

const (
	// FilterTypeBasic is BIP158 basic block filter (only type DogeGo builds).
	FilterTypeBasic byte = 0
	// MaxGetCFiltersRange limits blocks served per getcfilters (Core uses 1000).
	MaxGetCFiltersRange = 1000
)

// GetCFiltersPayload is the BIP157 getcfilters message body.
type GetCFiltersPayload = FilterRangeRequest

// CFilterPayload is the BIP157 cfilter message body.
type CFilterPayload struct {
	BlockHashLE [32]byte
	FilterType  byte
	Filter      []byte
	NumElements uint32
}

// EncodeCFilterPayload serializes cfilter for P2P.
func EncodeCFilterPayload(p CFilterPayload) ([]byte, error) {
	var b bytes.Buffer
	if _, err := b.Write(p.BlockHashLE[:]); err != nil {
		return nil, err
	}
	if err := b.WriteByte(p.FilterType); err != nil {
		return nil, err
	}
	if err := WriteCompactSize(&b, uint64(len(p.Filter))); err != nil {
		return nil, err
	}
	if len(p.Filter) > 0 {
		if _, err := b.Write(p.Filter); err != nil {
			return nil, err
		}
	}
	if err := binary.Write(&b, binary.LittleEndian, p.NumElements); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
