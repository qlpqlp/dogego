// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/binary"
)

// FilterRowsByScriptSet returns UTXO rows whose pkScript is in set without sorting the full chain set.
func (u *UtxoCache) FilterRowsByScriptSet(scriptSet map[string]struct{}, maxResults int) []UtxoDumpRow {
	if u == nil || len(scriptSet) == 0 {
		return nil
	}
	u.mu.RLock()
	defer u.mu.RUnlock()
	out := make([]UtxoDumpRow, 0, minInt(len(scriptSet)*4, 64))
	for k, e := range u.coins {
		if _, ok := scriptSet[string(e.PkScript)]; !ok {
			continue
		}
		var h [32]byte
		copy(h[:], k[:32])
		out = append(out, UtxoDumpRow{
			TxID:     displayTxidFromLE(h),
			Vout:     binary.LittleEndian.Uint32(k[32:]),
			Value:    e.Value,
			Height:   e.Height,
			PkScript: append([]byte(nil), e.PkScript...),
		})
		if maxResults > 0 && len(out) >= maxResults {
			break
		}
	}
	return out
}

// SumRowsMatching sums value/count for rows where match(pkScript) is true (no full sort/copy).
func (u *UtxoCache) SumRowsMatching(match func(pkScript []byte) bool) (totalKoinu int64, count int) {
	if u == nil || match == nil {
		return 0, 0
	}
	u.mu.RLock()
	defer u.mu.RUnlock()
	for _, e := range u.coins {
		if match(e.PkScript) {
			totalKoinu += e.Value
			count++
		}
	}
	return totalKoinu, count
}

// ForEachRow visits every UTXO without sorting. fn returns false to stop early.
func (u *UtxoCache) ForEachRow(fn func(row UtxoDumpRow) bool) {
	if u == nil || fn == nil {
		return
	}
	u.mu.RLock()
	defer u.mu.RUnlock()
	for k, e := range u.coins {
		var h [32]byte
		copy(h[:], k[:32])
		row := UtxoDumpRow{
			TxID:     displayTxidFromLE(h),
			Vout:     binary.LittleEndian.Uint32(k[32:]),
			Value:    e.Value,
			Height:   e.Height,
			PkScript: e.PkScript,
		}
		if !fn(row) {
			return
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
