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

// TxAtBlockMeta is header fields needed for getrawtransaction without full block decode.
type TxAtBlockMeta struct {
	Header primitives.BlockHeader
}

// ReadTxAtIndex returns transaction txIdx from a serialized block payload without retaining
// earlier transactions (Core getrawtransaction with -txindex still reads one tx from blk).
func ReadTxAtIndex(raw []byte, txIdx uint32) (*Tx, TxAtBlockMeta, error) {
	if len(raw) < 81 {
		return nil, TxAtBlockMeta{}, fmt.Errorf("block too short %d", len(raw))
	}
	var hdr primitives.BlockHeader
	if err := hdr.DecodeWire80(raw[:80]); err != nil {
		return nil, TxAtBlockMeta{}, err
	}
	r := bytes.NewReader(raw[80:])
	if isAuxPowVersion(hdr.Version) {
		if _, err := ReadAuxPow(r); err != nil {
			return nil, TxAtBlockMeta{}, fmt.Errorf("auxpow: %w", err)
		}
	}
	nTx, err := ReadCompactSize(r)
	if err != nil {
		return nil, TxAtBlockMeta{}, err
	}
	if txIdx >= uint32(nTx) {
		return nil, TxAtBlockMeta{}, fmt.Errorf("tx index %d past count %d", txIdx, nTx)
	}
	var tx *Tx
	for i := uint64(0); i < nTx; i++ {
		t, err := ReadTx(r)
		if err != nil {
			return nil, TxAtBlockMeta{}, fmt.Errorf("tx %d: %w", i, err)
		}
		if uint32(i) == txIdx {
			tx = t
			break
		}
	}
	if tx == nil {
		return nil, TxAtBlockMeta{}, fmt.Errorf("tx %d not found", txIdx)
	}
	return tx, TxAtBlockMeta{Header: hdr}, nil
}
