// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// HeaderHasAuxPowVersion reports whether the 80-byte header has the auxpow version bit set.
func HeaderHasAuxPowVersion(h80 []byte) bool {
	if len(h80) < 4 {
		return false
	}
	return isAuxPowVersion(int32(binary.LittleEndian.Uint32(h80[0:4])))
}

// ExtractAuxPowBytesFromBlock returns the serialized CAuxPow slice from a raw P2P block when present.
func ExtractAuxPowBytesFromBlock(raw []byte) ([]byte, bool, error) {
	if len(raw) < 80 {
		return nil, false, nil
	}
	if !HeaderHasAuxPowVersion(raw[:80]) {
		return nil, false, nil
	}
	r := bytes.NewReader(raw[80:])
	before := r.Len()
	if _, err := ReadAuxPow(r); err != nil {
		return nil, false, fmt.Errorf("auxpow extract: %w", err)
	}
	consumed := before - r.Len()
	if consumed <= 0 {
		return nil, false, fmt.Errorf("auxpow extract: empty")
	}
	return append([]byte(nil), raw[80:80+consumed]...), true, nil
}
