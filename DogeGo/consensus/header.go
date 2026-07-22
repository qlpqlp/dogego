// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"fmt"

	"dogego/chain"
	"dogego/pow"
)

// ValidateHeaderChain checks prev-hash linkage and 80-byte headers.
// If p.RelaxedPoW is true, PoW is skipped (unit tests / verifychain mock paths only; production nets use real scrypt).
// It does not enforce difficulty, auxpow, or contextual rules - use ValidateHeaders for P2P batches.
func ValidateHeaderChain(p chain.Params, prevTip [32]byte, headers [][]byte) error {
	cur := prevTip
	for i, hb := range headers {
		if len(hb) != 80 {
			return fmt.Errorf("header %d: want 80 bytes, got %d", i, len(hb))
		}
		verLE := binary.LittleEndian.Uint32(hb[0:4])
		if verLE&(1<<8) != 0 {
			return fmt.Errorf("header %d: auxpow headers require ValidateHeaders", i)
		}
		var prev [32]byte
		copy(prev[:], hb[4:36])
		if prev != cur {
			return fmt.Errorf("header %d: bad prev", i)
		}
		if !p.RelaxedPoW {
			bits := binary.LittleEndian.Uint32(hb[72:76])
			if err := pow.CheckScryptPoW(hb, bits); err != nil {
				return fmt.Errorf("header %d pow: %w", i, err)
			}
		}
		cur = pow.BlockHashLE(hb)
	}
	return nil
}
