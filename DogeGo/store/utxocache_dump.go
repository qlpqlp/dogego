// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/binary"
	"encoding/hex"
	"sort"
)

// UtxoDumpRow is one unspent output for dumptxoutset export.
type UtxoDumpRow struct {
	TxID     string
	Vout     uint32
	Value    int64
	Height   int64
	PkScript []byte
}

// DumpRows returns all cached UTXOs sorted by outpoint (txid, vout).
func (u *UtxoCache) DumpRows() []UtxoDumpRow {
	if u == nil {
		return nil
	}
	u.mu.RLock()
	defer u.mu.RUnlock()
	keys := make([][36]byte, 0, len(u.coins))
	for k := range u.coins {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return string(keys[i][:]) < string(keys[j][:])
	})
	out := make([]UtxoDumpRow, 0, len(keys))
	for _, k := range keys {
		e := u.coins[k]
		var h [32]byte
		copy(h[:], k[:32])
		out = append(out, UtxoDumpRow{
			TxID:     displayTxidFromLE(h),
			Vout:     binary.LittleEndian.Uint32(k[32:]),
			Value:    e.Value,
			Height:   e.Height,
			PkScript: append([]byte(nil), e.PkScript...),
		})
	}
	return out
}

func displayTxidFromLE(h [32]byte) string {
	b := make([]byte, 32)
	for i := 0; i < 32; i++ {
		b[i] = h[31-i]
	}
	return hex.EncodeToString(b)
}
