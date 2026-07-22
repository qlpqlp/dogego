// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"encoding/binary"
	"fmt"
)

// DecodeFeeFilterPayload parses the P2P "feefilter" body: 8-byte little-endian minimum fee rate
// (satoshis per 1000 vbytes / per kB - same encoding as Bitcoin Core / Dogecoin).
func DecodeFeeFilterPayload(payload []byte) (uint64, error) {
	if len(payload) != 8 {
		return 0, fmt.Errorf("feefilter: want 8 bytes, got %d", len(payload))
	}
	return binary.LittleEndian.Uint64(payload), nil
}

// EncodeFeeFilterPayload builds the P2P feefilter message body (8-byte LE koinu per kB).
func EncodeFeeFilterPayload(koinuPerKB uint64) []byte {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], koinuPerKB)
	return b[:]
}
