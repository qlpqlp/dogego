// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// bitWriter accumulates bits MSB-first within each byte (Bitcoin blockfilter bitstream).
type bitWriter struct {
	buf []byte
	bit uint8 // pending bits in low bits of bit, count in high nibble via len tracking
	n   int   // number of bits in pending (0-7)
}

func (w *bitWriter) writeBit(v bool) {
	if v {
		w.bit |= 1 << (7 - w.n)
	}
	w.n++
	if w.n == 8 {
		w.buf = append(w.buf, w.bit)
		w.bit = 0
		w.n = 0
	}
}

func (w *bitWriter) writeBits(v uint64, n int) {
	for i := n - 1; i >= 0; i-- {
		w.writeBit((v>>uint(i))&1 == 1)
	}
}

func (w *bitWriter) flush() []byte {
	if w.n > 0 {
		w.buf = append(w.buf, w.bit)
	}
	return w.buf
}
