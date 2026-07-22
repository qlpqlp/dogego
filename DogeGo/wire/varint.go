// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"encoding/binary"
	"io"
)

// WriteCompactSize writes Bitcoin compact size to w.
func WriteCompactSize(w io.Writer, n uint64) error {
	if n < 253 {
		_, err := w.Write([]byte{byte(n)})
		return err
	}
	if n <= 0xffff {
		if _, err := w.Write([]byte{253}); err != nil {
			return err
		}
		return binary.Write(w, binary.LittleEndian, uint16(n))
	}
	if n <= 0xffffffff {
		if _, err := w.Write([]byte{254}); err != nil {
			return err
		}
		return binary.Write(w, binary.LittleEndian, uint32(n))
	}
	if _, err := w.Write([]byte{255}); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, n)
}

// ReadCompactSize reads Bitcoin compact size from r.
func ReadCompactSize(r io.Reader) (uint64, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	switch b[0] {
	case 0xff:
		var v uint64
		if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
			return 0, err
		}
		return v, nil
	case 0xfe:
		var v uint32
		if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
			return 0, err
		}
		return uint64(v), nil
	case 0xfd:
		var v uint16
		if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
			return 0, err
		}
		return uint64(v), nil
	default:
		return uint64(b[0]), nil
	}
}

// ReadVarInt reads variable-length integer (same encoding as compact size in Dogecoin wire).
func ReadVarInt(r io.Reader) (uint64, error) {
	return ReadCompactSize(r)
}

// WriteVarInt writes variable-length integer.
func WriteVarInt(w io.Writer, n uint64) error {
	return WriteCompactSize(w, n)
}
