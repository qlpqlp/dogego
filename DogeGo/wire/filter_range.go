// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	// MaxGetCFHeadersRange is the maximum blocks per getcfheaders/cfheaders (BIP157).
	MaxGetCFHeadersRange = 2000
	// CFCheckptInterval is the block height gap between cfcheckpt filter headers.
	CFCheckptInterval = 1000
)

// FilterRangeRequest is the shared body for getcfilters and getcfheaders.
type FilterRangeRequest struct {
	FilterType  byte
	StartHeight uint32
	StopHashLE  [32]byte
}

// DecodeFilterRangeRequest parses getcfilters/getcfheaders (filter_type, start_height, stop_hash).
func DecodeFilterRangeRequest(pl []byte) (FilterRangeRequest, error) {
	var out FilterRangeRequest
	if len(pl) < 37 {
		return out, fmt.Errorf("filter range: short payload")
	}
	r := bytes.NewReader(pl)
	if err := binary.Read(r, binary.LittleEndian, &out.FilterType); err != nil {
		return out, err
	}
	if err := binary.Read(r, binary.LittleEndian, &out.StartHeight); err != nil {
		return out, err
	}
	if _, err := io.ReadFull(r, out.StopHashLE[:]); err != nil {
		return out, err
	}
	if r.Len() != 0 {
		return out, fmt.Errorf("filter range: trailing bytes")
	}
	return out, nil
}

// DecodeGetCFiltersPayload is an alias for DecodeFilterRangeRequest.
func DecodeGetCFiltersPayload(pl []byte) (GetCFiltersPayload, error) {
	req, err := DecodeFilterRangeRequest(pl)
	return GetCFiltersPayload(req), err
}
