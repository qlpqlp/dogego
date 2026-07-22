// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/binary"
	"fmt"

	"github.com/klauspost/compress/zstd"
)

// On-disk block record (per-file or bundled append):
//   magic u32 (0x0CB16ED0) | uncompressed u32 | stored u32 | hash[32] | payload[stored]

const (
	blockRecordMagic     = uint32(0x0CB16ED0)
	blockRecordHeaderLen = 4 + 4 + 4 + 32
)

var (
	zstdEnc, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	zstdDec, _ = zstd.NewReader(nil)
)

func encodeBlockRecord(hashLE [32]byte, raw []byte, useZstd bool) ([]byte, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty block payload")
	}
	stored := raw
	if useZstd {
		if zstdEnc == nil {
			var err error
			zstdEnc, err = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
			if err != nil {
				return nil, err
			}
		}
		comp := zstdEnc.EncodeAll(raw, make([]byte, 0, len(raw)/2))
		if len(comp) < len(raw) {
			stored = comp
		}
	}
	out := make([]byte, blockRecordHeaderLen+len(stored))
	binary.LittleEndian.PutUint32(out[0:4], blockRecordMagic)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(raw)))
	binary.LittleEndian.PutUint32(out[8:12], uint32(len(stored)))
	copy(out[12:44], hashLE[:])
	copy(out[44:], stored)
	return out, nil
}

func decodeBlockRecord(rec []byte, expectHash [32]byte) ([]byte, error) {
	if len(rec) < blockRecordHeaderLen {
		return nil, fmt.Errorf("block record too short %d", len(rec))
	}
	if binary.LittleEndian.Uint32(rec[0:4]) != blockRecordMagic {
		return nil, fmt.Errorf("block record bad magic")
	}
	uncompressed := binary.LittleEndian.Uint32(rec[4:8])
	storedLen := binary.LittleEndian.Uint32(rec[8:12])
	var got [32]byte
	copy(got[:], rec[12:44])
	if got != expectHash {
		return nil, fmt.Errorf("block record hash mismatch")
	}
	if int(storedLen) != len(rec)-blockRecordHeaderLen {
		return nil, fmt.Errorf("block record length mismatch")
	}
	stored := rec[blockRecordHeaderLen:]
	if uint32(len(stored)) == uncompressed {
		return append([]byte(nil), stored...), nil
	}
	if zstdDec == nil {
		var err error
		zstdDec, err = zstd.NewReader(nil)
		if err != nil {
			return nil, err
		}
	}
	raw, err := zstdDec.DecodeAll(stored, nil)
	if err != nil {
		return nil, fmt.Errorf("block zstd decode: %w", err)
	}
	if uint32(len(raw)) != uncompressed {
		return nil, fmt.Errorf("block zstd size mismatch")
	}
	return raw, nil
}

// isPlainLegacyBin reports whether bytes look like an uncompressed legacy .bin file (no record header).
func isPlainLegacyBin(b []byte) bool {
	if len(b) < 80 {
		return false
	}
	return binary.LittleEndian.Uint32(b[0:4]) != blockRecordMagic
}
