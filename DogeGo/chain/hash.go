// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import (
	"encoding/hex"
	"fmt"
)

// Hash256FromDisplayHex decodes a 64-digit Core-style GetHex() string into 32-byte
// little-endian wire order (uint256 serialization) used in block headers.
func Hash256FromDisplayHex(display string) ([32]byte, error) {
	var out [32]byte
	b, err := hex.DecodeString(display)
	if err != nil || len(b) != 32 {
		return out, fmt.Errorf("invalid 32-byte hex")
	}
	for i := 0; i < 32; i++ {
		out[i] = b[31-i]
	}
	return out, nil
}
