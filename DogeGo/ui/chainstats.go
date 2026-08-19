// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/binary"
	"sort"
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/rpc"
	"dogego/store"
	"dogego/wire"
)

const (
	dogeTargetSpacingSec = 60.0
	maxHeaderWalk        = 2000
	maxRawBodyScan       = 400
	maxHeaderWalkLight  = 1500
	maxRawBodyScanLight = 200
	windowSecs           = 86400
)

type minerWindowScan struct {
	heights   []int64
	miners    map[string]int
	addrSet   map[string]struct{}
	minted    int64
	rawHits   int
	rawMiss   int
	parseErrs int
}

// BuildChainStats derives dashboard metrics from the local header journal and optional raw blocks.
// chainActiveHint is Core chainActive (-1 unknown); storedBodiesHint is contiguous raw bodies (-1 unknown).
// When light is true, cap header walks and body scans (used during IBD to avoid UI stalls) but still
// scan stored coinbases so miner distribution can render from the tipâ€™s ~24h header-time window.
func BuildChainStats(j *store.HeaderJournal, raw *store.RawBlockStore, addrVer byte, now time.Time, chainActiveHint, storedBodiesHint int64, light bool) map[string]any {
	out := map[string]any{
		"dogego_note": "Estimates from this node only. Hashrate uses tip difficulty Ã- 2Â³Â² / 60s (Dogecoin target spacing). Miner/address stats scan stored raw blocks in a ~24h header-time window (capped).",
	}
	if j == nil {
		out["error"] = "no journal"
		return out
	}
	headerTip, err := j.TipHeight()
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	chainActive := chainActiveHint
	if chainActive < 0 {
		chainActive = rpc.ActiveChainBlockHeight(j, raw)
	}
	storedBodies := storedBodiesHint
	out["tip_height"] = headerTip
	out["chain_active_height"] = chainActive
	if storedBodies >= 0 {
		out["stored_bodies_height"] = storedBodies
	}
	statsTip := chainActive
	if statsTip < 0 {
		statsTip = headerTip
	}
	hTip, err := j.ReadHeaderAt(statsTip)
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	tipTime := int64(binary.LittleEndian.Uint32(hTip[68:72]))
	out["tip_header_time"] = tipTime
	bitsTip := binary.LittleEndian.Uint32(hTip[72:76])
	if d, err := pow.DifficultyFromCompact(bitsTip); err == nil {
		out["tip_difficulty"] = d
		// Expected network hashrate order-of-magnitude from difficulty (scrypt chain, ~60s target).
		out["estimated_network_hashrate_hs"] = d * float64(1<<32) / dogeTargetSpacingSec
	}
	// Mean header delta over last up to 120 blocks (smoothed inter-arrival).
	const span = 120
	if statsTip >= 1 {
		var sumDt int64
		var n int64
		start := statsTip - span
		if start < 1 {
			start = 1
		}
		var prevT int64 = -1
		for h := start; h <= statsTip; h++ {
			b, err := j.ReadHeaderAt(h)
			if err != nil {
				break
			}
			t := int64(binary.LittleEndian.Uint32(b[68:72]))
			if prevT >= 0 && t >= prevT {
				sumDt += t - prevT
				n++
			}
			prevT = t
		}
		if n > 0 {
			out["mean_header_delta_sec_last"] = float64(sumDt) / float64(n)
			out["mean_header_delta_sample_blocks"] = n
		}
	}

	headerWalk := int64(maxHeaderWalk)
	bodyScanCap := maxRawBodyScan
	if light {
		headerWalk = maxHeaderWalkLight
		bodyScanCap = maxRawBodyScanLight
		out["dogego_light"] = true
	}
	out["window_header_hours"] = windowSecs / 3600

	windowTip := statsTip
	scan := scanMinerWindow(j, raw, addrVer, windowTip, headerWalk, bodyScanCap)
	// Headers/connect tip may sit ahead of stored bodies during IBD â€” fall back to the highest
	// contiguous stored body so the chart still reflects that tipâ€™s historical ~24h window.
	if scan.rawHits == 0 && storedBodies >= 0 && storedBodies < statsTip {
		fb := scanMinerWindow(j, raw, addrVer, storedBodies, headerWalk, bodyScanCap)
		if fb.rawHits > 0 {
			scan = fb
			windowTip = storedBodies
			out["miner_window_fallback"] = true
		}
	}
	out["miner_window_tip_height"] = windowTip
	out["approx_window_blocks"] = len(scan.heights)
	out["raw_blocks_in_window"] = scan.rawHits
	out["raw_blocks_missing_in_window"] = scan.rawMiss
	out["raw_parse_errors_in_window"] = scan.parseErrs
	out["unique_p2pkh_in_coinbase_window"] = len(scan.addrSet)
	out["minted_in_scanned_raw_koinu"] = scan.minted
	out["minted_in_scanned_raw_doge"] = float64(scan.minted) / 1e8

	type minerRow struct {
		Address string `json:"address"`
		Blocks  int    `json:"blocks"`
	}
	var rows []minerRow
	for a, n := range scan.miners {
		rows = append(rows, minerRow{Address: a, Blocks: n})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Blocks == rows[j].Blocks {
			return rows[i].Address < rows[j].Address
		}
		return rows[i].Blocks > rows[j].Blocks
	})
	if len(rows) > 12 {
		rows = rows[:12]
	}
	out["top_miners_by_payout_p2pkh"] = rows
	distinctMiners := len(scan.miners)
	out["distinct_miner_slots"] = distinctMiners
	if scan.rawHits > 0 && distinctMiners == 1 {
		out["miner_concentration_note"] = "all scanned coinbases mapped to one P2PKH bucket (or only non-P2PKH)"
	} else if scan.rawHits > 0 && distinctMiners > 1 {
		out["miner_concentration_note"] = "multiple payout addresses observed in scanned window"
	}
	return out
}

func scanMinerWindow(j *store.HeaderJournal, raw *store.RawBlockStore, addrVer byte, tip, headerWalk int64, bodyScanCap int) minerWindowScan {
	out := minerWindowScan{
		miners:  map[string]int{},
		addrSet: map[string]struct{}{},
	}
	if j == nil || tip < 0 {
		return out
	}
	hTip, err := j.ReadHeaderAt(tip)
	if err != nil {
		return out
	}
	tipTime := int64(binary.LittleEndian.Uint32(hTip[68:72]))
	threshold := tipTime - windowSecs
	minH := int64(0)
	if tip > headerWalk {
		minH = tip - headerWalk
	}
	for h := tip; h >= minH; h-- {
		b, err := j.ReadHeaderAt(h)
		if err != nil {
			break
		}
		t := int64(binary.LittleEndian.Uint32(b[68:72]))
		out.heights = append(out.heights, h)
		if t < threshold {
			break
		}
	}
	if raw == nil {
		return out
	}
	for i := 0; i < len(out.heights) && i < bodyScanCap; i++ {
		h := out.heights[i]
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			continue
		}
		id := pow.BlockHashLE(h80)
		if !raw.Has(id) {
			out.rawMiss++
			continue
		}
		payload, err := raw.Get(id)
		if err != nil {
			out.rawMiss++
			continue
		}
		out.rawHits++
		cb, _, err := wire.ReadTxAtIndex(payload, 0)
		if err != nil {
			out.parseErrs++
			continue
		}
		if !isCoinbaseTx(cb) {
			continue
		}
		var firstAddr string
		for _, o := range cb.Vout {
			out.minted += o.Value
			a := chain.PayToPubKeyHashAddress(o.PkScript, addrVer)
			if a != "" {
				out.addrSet[a] = struct{}{}
				if firstAddr == "" {
					firstAddr = a
				}
			}
		}
		if firstAddr != "" {
			out.miners[firstAddr]++
		} else {
			out.miners["(non-P2PKH or empty)"]++
		}
	}
	return out
}
