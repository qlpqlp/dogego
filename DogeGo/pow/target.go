// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pow

import (
	"encoding/hex"
	"fmt"
	"math/big"
)

// CompactToTargetBig interprets nBits as a 256-bit proof-of-work target (Core SetCompact).
func CompactToTargetBig(bits uint32) *big.Int {
	nSize := bits >> 24
	mantissa := bits & 0x007fffff
	if nSize <= 3 {
		return new(big.Int).Lsh(big.NewInt(int64(mantissa)), uint(8*(3-nSize)))
	}
	return new(big.Int).Lsh(big.NewInt(int64(mantissa)), uint(8*(nSize-3)))
}

// TargetHexFromCompact returns the GBT-style 64-char hex target (big-endian display).
func TargetHexFromCompact(bits uint32) string {
	t := CompactToTargetBig(bits)
	b := t.Bytes()
	out := make([]byte, 32)
	for i := 0; i < len(b) && i < 32; i++ {
		out[31-i] = b[len(b)-1-i]
	}
	return hex.EncodeToString(out)
}

// BitsHex returns nBits as 8 hex digits (little-endian wire order).
func BitsHex(bits uint32) string {
	return fmt.Sprintf("%08x", bits)
}
