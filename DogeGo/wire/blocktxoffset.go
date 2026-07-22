// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"fmt"

	"dogego/primitives"
)

// BlockTxDiskOffsets returns byte offsets of each transaction within a full block payload
// (same coordinate system as Core CDiskTxPos::nTxOffset after the 80-byte header).
func BlockTxDiskOffsets(raw []byte) ([]uint32, error) {
	if len(raw) < 81 {
		return nil, fmt.Errorf("block too short %d", len(raw))
	}
	var hdr primitives.BlockHeader
	if err := hdr.DecodeWire80(raw[:80]); err != nil {
		return nil, err
	}
	r := bytes.NewReader(raw[80:])
	if isAuxPowVersion(hdr.Version) {
		if _, err := ReadAuxPow(r); err != nil {
			return nil, fmt.Errorf("auxpow: %w", err)
		}
	}
	nTx, err := ReadCompactSize(r)
	if err != nil {
		return nil, err
	}
	if nTx == 0 {
		return nil, fmt.Errorf("zero transactions")
	}
	offsets := make([]uint32, 0, nTx)
	for i := uint64(0); i < nTx; i++ {
		offsets = append(offsets, uint32(len(raw)-r.Len()))
		if _, err := ReadTx(r); err != nil {
			return nil, fmt.Errorf("tx %d: %w", i, err)
		}
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("trailing %d bytes after txs", r.Len())
	}
	return offsets, nil
}
