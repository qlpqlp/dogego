// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/binary"
	"fmt"
)

// Tx index on-disk entry (per txid file under indexes/tx/):
//   legacy (36 bytes): block hash LE + tx position uint32
//   v2 (>36 bytes):    same 36-byte prefix + serialized tx bytes (Core -txindex style fast lookup)

const txIndexMetaLen = 36

// TxIndexHit is the result of a confirmed txid lookup.
type TxIndexHit struct {
	BlockHashLE [32]byte
	TxIndex     uint32
	TxRaw       []byte // non-nil when v2 entry stores serialized tx
}

func encodeTxIndexEntry(blockHashLE [32]byte, txIndex uint32, txRaw []byte) []byte {
	out := make([]byte, txIndexMetaLen+len(txRaw))
	copy(out[:32], blockHashLE[:])
	binary.LittleEndian.PutUint32(out[32:], txIndex)
	if len(txRaw) > 0 {
		copy(out[txIndexMetaLen:], txRaw)
	}
	return out
}

func decodeTxIndexEntry(data []byte) (TxIndexHit, error) {
	var hit TxIndexHit
	if len(data) < txIndexMetaLen {
		return hit, fmt.Errorf("corrupt tx index entry (len %d)", len(data))
	}
	copy(hit.BlockHashLE[:], data[:32])
	hit.TxIndex = binary.LittleEndian.Uint32(data[32:])
	if len(data) > txIndexMetaLen {
		hit.TxRaw = append([]byte(nil), data[txIndexMetaLen:]...)
	}
	return hit, nil
}
