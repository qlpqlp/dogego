// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pqcrypto

import (
	"crypto/sha256"
	"errors"
	"io"
)

var errWant32 = errors.New("pqcrypto: message must be 32 bytes")

type detReader struct {
	seed [32]byte
	pos  int
}

func deterministicReader(seed []byte) io.Reader {
	var s [32]byte
	if len(seed) >= 32 {
		copy(s[:], seed[:32])
	} else {
		h := sha256.Sum256(seed)
		copy(s[:], h[:])
	}
	return &detReader{seed: s}
}

func (d *detReader) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		h := sha256.New()
		_, _ = h.Write(d.seed[:])
		var ctr [4]byte
		ctr[0] = byte(d.pos)
		ctr[1] = byte(d.pos >> 8)
		ctr[2] = byte(d.pos >> 16)
		ctr[3] = byte(d.pos >> 24)
		_, _ = h.Write(ctr[:])
		block := h.Sum(nil)
		d.pos++
		copied := copy(p[n:], block)
		n += copied
	}
	return n, nil
}
