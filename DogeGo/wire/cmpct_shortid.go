// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

const cmpctShortTxIDBytes = 6

// CmpctShortTxID returns the BIP152 v1 six-byte short transaction ID for a legacy tx hash.
func CmpctShortTxID(header80 []byte, nonce uint64, txid [32]byte) uint64 {
	if len(header80) < 80 {
		return 0
	}
	header80 = header80[:80]
	buf := make([]byte, 0, 88)
	buf = append(buf, header80...)
	var nb [8]byte
	binary.LittleEndian.PutUint64(nb[:], nonce)
	buf = append(buf, nb[:]...)
	h := sha256.Sum256(buf)
	k0 := binary.LittleEndian.Uint64(h[0:8])
	k1 := binary.LittleEndian.Uint64(h[8:16])
	sip := sipHash24(k0, k1, txid[:])
	return sip & 0x0000FFFFFFFFFFFF
}

// EncodeCmpctShortIDs serializes short IDs as little-endian six-byte integers.
func EncodeCmpctShortIDs(ids []uint64) []byte {
	out := make([]byte, 0, len(ids)*cmpctShortTxIDBytes)
	for _, id := range ids {
		id &= 0x0000FFFFFFFFFFFF
		for i := 0; i < cmpctShortTxIDBytes; i++ {
			out = append(out, byte(id>>(8*i)))
		}
	}
	return out
}

// DecodeCmpctShortIDs parses short IDs from wire bytes (must be a multiple of six).
func DecodeCmpctShortIDs(raw []byte) ([]uint64, error) {
	if len(raw)%cmpctShortTxIDBytes != 0 {
		return nil, fmt.Errorf("cmpct shortids: invalid length %d", len(raw))
	}
	n := len(raw) / cmpctShortTxIDBytes
	out := make([]uint64, n)
	for i := 0; i < n; i++ {
		off := i * cmpctShortTxIDBytes
		var buf [8]byte
		copy(buf[:cmpctShortTxIDBytes], raw[off:off+cmpctShortTxIDBytes])
		out[i] = binary.LittleEndian.Uint64(buf[:]) & 0x0000FFFFFFFFFFFF
	}
	return out, nil
}
