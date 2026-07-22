// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/binary"
	"sort"

	"dogego/chain"
	"dogego/store"
)

const topUtxoHolderLimit = 15

type utxoHolderAgg struct {
	address   string
	koinu     int64
	utxoCount int
	minHeight int64
	maxHeight int64
}

// BuildTopUtxoHolders ranks addresses by unspent DOGE in the live UTXO cache.
func BuildTopUtxoHolders(u *store.UtxoCache, j *store.HeaderJournal, addrIx *store.AddrIndex, pubVer, scriptVer byte, maxRows int) []map[string]any {
	if u == nil || maxRows <= 0 {
		return nil
	}
	byAddr := make(map[string]*utxoHolderAgg)
	u.ForEachRow(func(row store.UtxoDumpRow) bool {
		addr := chain.ScriptPubKeyAddress(row.PkScript, pubVer, scriptVer)
		if addr == "" {
			return true
		}
		agg := byAddr[addr]
		if agg == nil {
			agg = &utxoHolderAgg{address: addr, minHeight: row.Height, maxHeight: row.Height}
			byAddr[addr] = agg
		}
		agg.koinu += row.Value
		agg.utxoCount++
		if row.Height >= 0 {
			if agg.minHeight < 0 || row.Height < agg.minHeight {
				agg.minHeight = row.Height
			}
			if row.Height > agg.maxHeight {
				agg.maxHeight = row.Height
			}
		}
		return true
	})
	if len(byAddr) == 0 {
		return nil
	}
	rows := make([]*utxoHolderAgg, 0, len(byAddr))
	for _, agg := range byAddr {
		rows = append(rows, agg)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].koinu == rows[j].koinu {
			return rows[i].address < rows[j].address
		}
		return rows[i].koinu > rows[j].koinu
	})
	if len(rows) > maxRows {
		rows = rows[:maxRows]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, agg := range rows {
		row := map[string]any{
			"address":    agg.address,
			"doge":       float64(agg.koinu) / 1e8,
			"utxo_count": agg.utxoCount,
		}
		if agg.minHeight >= 0 {
			row["oldest_utxo_height"] = agg.minHeight
			if ts := headerTimeAt(j, agg.minHeight); ts > 0 {
				row["first_seen_time"] = ts
			}
		}
		if agg.maxHeight >= 0 {
			row["newest_utxo_height"] = agg.maxHeight
			if ts := headerTimeAt(j, agg.maxHeight); ts > 0 {
				row["last_seen_time"] = ts
			}
		}
		if addrIx != nil && addrIx.HasAny() {
			if h160, ok := store.Hash160FromAddress(agg.address); ok {
				enrichHolderAddrIndex(row, addrIx, h160, j)
			}
		}
		out = append(out, row)
	}
	return out
}

func enrichHolderAddrIndex(row map[string]any, addrIx *store.AddrIndex, h160 [20]byte, j *store.HeaderJournal) {
	recvTotal := 0
	if _, total, err := addrIx.LookupReceives(h160, 0, 1); err == nil && total > 0 {
		recvTotal = total
		if hits, _, err := addrIx.LookupReceives(h160, 0, 1); err == nil && len(hits) > 0 {
			if ts := headerTimeAt(j, hits[0].Height); ts > 0 {
				row["last_tx_time"] = ts
			}
		}
		if hits, _, err := addrIx.LookupReceives(h160, total-1, 1); err == nil && len(hits) > 0 {
			if ts := headerTimeAt(j, hits[0].Height); ts > 0 {
				row["first_tx_time"] = ts
			}
		}
	}
	if spends, total, err := addrIx.LookupSpends(h160, 0, 1); err == nil && len(spends) > 0 {
		if ts := headerTimeAt(j, spends[0].Height); ts > 0 {
			if cur, ok := row["last_tx_time"].(int64); !ok || ts > cur {
				row["last_tx_time"] = ts
			}
		}
		_ = total
	}
	_ = recvTotal
}

func headerTimeAt(j *store.HeaderJournal, height int64) int64 {
	if j == nil || height < 0 {
		return 0
	}
	h80, err := j.ReadHeaderAt(height)
	if err != nil || len(h80) < 72 {
		return 0
	}
	return int64(binary.LittleEndian.Uint32(h80[68:72]))
}
