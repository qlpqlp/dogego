// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package primitives

import (
	"encoding/binary"
	"errors"
)

var errLen80 = errors.New("primitives: header must be 80 bytes")

// BlockHeader is a non-auxpow 80-byte header view.
type BlockHeader struct {
	Version    int32
	PrevBlock  [32]byte
	MerkleRoot [32]byte
	Timestamp  uint32
	Bits       uint32
	Nonce      uint32
}

// DecodeWire80 fills h from 80-byte wire serialization.
func (h *BlockHeader) DecodeWire80(b []byte) error {
	if len(b) != 80 {
		return errLen80
	}
	h.Version = int32(binary.LittleEndian.Uint32(b[0:4]))
	copy(h.PrevBlock[:], b[4:36])
	copy(h.MerkleRoot[:], b[36:68])
	h.Timestamp = binary.LittleEndian.Uint32(b[68:72])
	h.Bits = binary.LittleEndian.Uint32(b[72:76])
	h.Nonce = binary.LittleEndian.Uint32(b[76:80])
	return nil
}

// EncodeWire80 returns the 80-byte serialization (non-auxpow).
func (h *BlockHeader) EncodeWire80() [80]byte {
	var b [80]byte
	binary.LittleEndian.PutUint32(b[0:4], uint32(h.Version))
	copy(b[4:36], h.PrevBlock[:])
	copy(b[36:68], h.MerkleRoot[:])
	binary.LittleEndian.PutUint32(b[68:72], h.Timestamp)
	binary.LittleEndian.PutUint32(b[72:76], h.Bits)
	binary.LittleEndian.PutUint32(b[76:80], h.Nonce)
	return b
}
