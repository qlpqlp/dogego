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

// Inventory type flags (subset of src/protocol.h).
const (
	InvTypeTx            = 1
	InvTypeBlock         = 2
	InvTypeFilteredBlock = 3
	InvTypeCmpctBlock    = 4
	InvWitnessFlag       = 1 << 30
	InvTypeWitnessBlock  = InvTypeBlock | InvWitnessFlag
	InvTypeWitnessTx     = InvTypeTx | InvWitnessFlag
)

// InvEntry is one CInv (type + 32-byte hash LE wire order).
type InvEntry struct {
	Type uint32
	Hash [32]byte
}

// DecodeInvPayload parses the body of an "inv" or "getdata" message (vector<CInv>).
func DecodeInvPayload(payload []byte) ([]InvEntry, error) {
	r := bytes.NewReader(payload)
	n, err := ReadCompactSize(r)
	if err != nil {
		return nil, err
	}
	if n > 50000 {
		return nil, fmt.Errorf("inv too large %d", n)
	}
	out := make([]InvEntry, 0, n)
	for i := uint64(0); i < n; i++ {
		var e InvEntry
		if err := binary.Read(r, binary.LittleEndian, &e.Type); err != nil {
			return nil, err
		}
		if _, err := io.ReadFull(r, e.Hash[:]); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("inv trailing %d bytes", r.Len())
	}
	return out, nil
}

// EncodeInvPayload builds an "inv" message payload (same vector<CInv> layout as getdata).
func EncodeInvPayload(inv []InvEntry) ([]byte, error) {
	return EncodeGetData(inv)
}

// EncodeGetData builds a getdata message payload (vector<CInv>).
func EncodeGetData(inv []InvEntry) ([]byte, error) {
	if len(inv) > 50000 {
		return nil, fmt.Errorf("getdata inv too large %d", len(inv))
	}
	var buf bytes.Buffer
	if err := WriteCompactSize(&buf, uint64(len(inv))); err != nil {
		return nil, err
	}
	for _, e := range inv {
		if err := binary.Write(&buf, binary.LittleEndian, e.Type); err != nil {
			return nil, err
		}
		if _, err := buf.Write(e.Hash[:]); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}
