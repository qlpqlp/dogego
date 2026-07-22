// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"fmt"
	"io"
)

// ReadCompactSizeDifferential reads a BIP152 differentially encoded compact size index.
// prevIndex is the previous absolute index, or -1 before the first entry.
func ReadCompactSizeDifferential(r io.Reader, prevIndex int64) (uint64, error) {
	delta, err := ReadCompactSize(r)
	if err != nil {
		return 0, err
	}
	return uint64(prevIndex + int64(delta) + 1), nil
}

// WriteCompactSizeDifferential writes a BIP152 differentially encoded compact size index.
func WriteCompactSizeDifferential(w io.Writer, index uint64, prevIndex int64) error {
	if int64(index) <= prevIndex {
		return fmt.Errorf("cmpct differential index out of order: %d after %d", index, prevIndex)
	}
	return WriteCompactSize(w, uint64(int64(index)-prevIndex-1))
}
