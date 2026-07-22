// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"fmt"

	"dogego/store"
)

// StoredHeaderRangeNeedsAux reports whether any header in [startHeight,endHeight] uses the auxpow version bit.
func StoredHeaderRangeNeedsAux(j *store.HeaderJournal, startHeight, endHeight int64) (bool, error) {
	if j == nil {
		return false, nil
	}
	for h := startHeight; h <= endHeight; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return false, err
		}
		if len(h80) != 80 {
			return false, fmt.Errorf("height %d: bad header len", h)
		}
		if isAuxpowVersionU(binary.LittleEndian.Uint32(h80[0:4])) {
			return true, nil
		}
	}
	return false, nil
}
