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

// SerializeBlock encodes a parsed block as a P2P "block" message body.
func SerializeBlock(pb *ParsedBlock) ([]byte, error) {
	if pb == nil {
		return nil, fmt.Errorf("nil block")
	}
	var b bytes.Buffer
	h80 := pb.Header.EncodeWire80()
	if _, err := b.Write(h80[:]); err != nil {
		return nil, err
	}
	if isAuxPowVersion(pb.Header.Version) {
		if pb.Aux == nil {
			return nil, fmt.Errorf("auxpow block missing aux blob")
		}
		ab, err := SerializeAuxPow(pb.Aux)
		if err != nil {
			return nil, err
		}
		if _, err := b.Write(ab); err != nil {
			return nil, err
		}
	}
	if len(pb.Txs) == 0 {
		return nil, fmt.Errorf("block has no transactions")
	}
	if err := WriteCompactSize(&b, uint64(len(pb.Txs))); err != nil {
		return nil, err
	}
	for i, tx := range pb.Txs {
		raw, err := tx.Serialize()
		if err != nil {
			return nil, fmt.Errorf("tx %d: %w", i, err)
		}
		if _, err := b.Write(raw); err != nil {
			return nil, err
		}
	}
	return b.Bytes(), nil
}

// SerializeBlockFromTxRaws builds a block payload from header, optional aux wire bytes, and serialized txs.
func SerializeBlockFromTxRaws(header80 [80]byte, auxWire []byte, txs [][]byte) ([]byte, error) {
	if len(txs) == 0 {
		return nil, fmt.Errorf("block has no transactions")
	}
	var b bytes.Buffer
	if _, err := b.Write(header80[:]); err != nil {
		return nil, err
	}
	if len(auxWire) > 0 {
		if _, err := b.Write(auxWire); err != nil {
			return nil, err
		}
	}
	if err := WriteCompactSize(&b, uint64(len(txs))); err != nil {
		return nil, err
	}
	for i, raw := range txs {
		if len(raw) == 0 {
			return nil, fmt.Errorf("tx %d empty", i)
		}
		if _, err := b.Write(raw); err != nil {
			return nil, err
		}
	}
	return b.Bytes(), nil
}
