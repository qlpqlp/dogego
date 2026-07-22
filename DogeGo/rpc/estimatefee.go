// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"

	"dogego/consensus"
)

// execEstimateSmartFee returns a Core-shaped object (DogeGo subset of Core fee estimation).
// Optional second param estimate_mode: "conservative" (default) or "economical".
func execEstimateSmartFee(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	nblocks := 6
	conservative := true
	if len(params) >= 1 {
		var v float64
		if err := json.Unmarshal(params[0], &v); err == nil && v >= 1 && v <= 1008 && v == float64(int64(v)) {
			nblocks = int(v)
		}
	}
	if len(params) >= 2 {
		var mode string
		if err := json.Unmarshal(params[1], &mode); err == nil {
			switch strings.ToLower(strings.TrimSpace(mode)) {
			case "economical", "eco":
				conservative = false
			case "conservative", "conservative_mode", "":
				conservative = true
			default:
				return nil, -8, "estimatesmartfee: estimate_mode must be conservative or economical"
			}
		}
	}
	rate, answerBlocks, errs := smartFeeKoinuPerKB(paths, nblocks, conservative)
	perKB := float64(rate) / 1e8
	if rate == 0 {
		perKB = -1
	}
	out := map[string]interface{}{
		"feerate":     perKB,
		"fee_rate":    perKB,
		"blocks":      answerBlocks,
		"errors":      errs,
		"dogego_note": "feerate from min relay + mempool/confirmed percentiles; conservative=max market, economical=min market",
	}
	if paths != nil && paths.FeeBucketEstimates != nil {
		if buckets := paths.FeeBucketEstimates(); len(buckets) > 0 {
			out["dogego_fee_buckets"] = buckets
		}
	}
	if paths != nil && paths.FeeBucketMarketStats != nil {
		if stats := paths.FeeBucketMarketStats(); len(stats) > 0 {
			out["dogego_fee_bucket_market"] = stats
		}
	}
	if paths != nil && paths.MempoolConfirmBucketStats != nil {
		if stats := paths.MempoolConfirmBucketStats(); len(stats) > 0 {
			out["dogego_mempool_confirm_buckets"] = stats
		}
	}
	if paths != nil && paths.MempoolLeftBucketStats != nil {
		if stats := paths.MempoolLeftBucketStats(); len(stats) > 0 {
			out["dogego_mempool_left_buckets"] = stats
		}
	}
	if paths != nil && paths.FeeConfirmStatsBucketMarket != nil {
		if stats := paths.FeeConfirmStatsBucketMarket(); len(stats) > 0 {
			out["dogego_fee_confirm_stats"] = stats
		}
	}
	return out, 0, ""
}

func smartFeeKoinuPerKB(paths *DataPaths, nblocks int, conservative bool) (rate uint64, answerBlocks int, errors []interface{}) {
	floor := minRelayFeeFromPaths(paths)
	if floor == 0 {
		floor = consensus.MinRelayTxFeePerKB()
	}
	if conservative {
		rate, answerBlocks, errors = smartFeeConservative(paths, nblocks, floor)
	} else {
		rate, answerBlocks, errors = smartFeeEconomical(paths, nblocks, floor)
	}
	if rate < floor {
		rate = floor
	}
	// Core estimateSmartFee: confTarget 1 is walked as 2; answer blocks reflect that target.
	if nblocks == 1 && (answerBlocks <= 1 || answerBlocks == 0) {
		answerBlocks = 2
	}
	return rate, answerBlocks, errors
}

func smartFeeConservative(paths *DataPaths, nblocks int, floor uint64) (rate uint64, answerBlocks int, errors []interface{}) {
	answerBlocks = nblocks
	rate = floor
	var hadMarket bool
	if paths != nil {
		if paths.MempoolFeeEstimateConservative != nil {
			if m := paths.MempoolFeeEstimateConservative(nblocks); m > 0 {
				hadMarket = true
				if m > rate {
					rate = m
				}
			}
		}
		if paths.FeeHistory != nil {
			if cs, ab := paths.FeeHistory.EstimateConfirmStatsSmart(nblocks, true); cs > 0 {
				hadMarket = true
				if cs > rate {
					rate = cs
				}
				if ab > 0 {
					answerBlocks = ab
				}
			}
		}
		if paths.ConfirmedFeeEstimateConservative != nil {
			if c := paths.ConfirmedFeeEstimateConservative(nblocks); c > 0 {
				hadMarket = true
				if c > rate {
					rate = c
				}
				if answerBlocks == nblocks {
					answerBlocks = consensus.ClosestStandardBucketTarget(nblocks)
				}
			}
		}
	}
	if !hadMarket {
		errors = append(errors, feeErrorEntry("INSUFFICIENT_FEE", "Insufficient data or no feerate found. Using minimum relay fee of the node."))
		answerBlocks = 0
	}
	return rate, answerBlocks, errors
}

func smartFeeEconomical(paths *DataPaths, nblocks int, floor uint64) (rate uint64, answerBlocks int, errors []interface{}) {
	answerBlocks = nblocks
	var market []uint64
	if paths != nil {
		if paths.MempoolFeeEstimateEconomical != nil {
			if m := paths.MempoolFeeEstimateEconomical(nblocks); m > 0 {
				market = append(market, m)
			}
		} else if paths.MempoolFeeEstimate != nil {
			if m := paths.MempoolFeeEstimate(nblocks); m > 0 {
				market = append(market, m)
			}
		}
		if paths.FeeHistory != nil {
			if cs, ab := paths.FeeHistory.EstimateConfirmStatsSmart(nblocks, false); cs > 0 {
				market = append(market, cs)
				if ab > 0 {
					answerBlocks = ab
				}
			}
		}
		if paths.ConfirmedFeeEstimate != nil {
			if c := paths.ConfirmedFeeEstimate(nblocks); c > 0 {
				market = append(market, c)
				if answerBlocks == nblocks {
					answerBlocks = consensus.ClosestStandardBucketTarget(nblocks)
				}
			}
		}
	}
	if min, ok := consensus.MinPositiveUint64(market...); ok {
		rate = min
		if rate < floor {
			rate = floor
		}
		return rate, answerBlocks, nil
	}
	errors = append(errors, feeErrorEntry("INSUFFICIENT_FEE", "Insufficient data or no feerate found. Using minimum relay fee of the node."))
	return floor, 0, errors
}

func feeErrorEntry(typ, message string) map[string]interface{} {
	return map[string]interface{}{
		"type":    typ,
		"message": message,
	}
}

// execEstimateRawFee returns a feerate for a hypothetical raw transaction (Core-shaped; uses smart fee estimator).
// Optional third param: hex unsigned tx to scale feerate hint by serialized size vs default 250-byte template.
func execEstimateRawFee(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	res, code, msg := execEstimateSmartFee(paths, params)
	if code != 0 {
		return nil, code, msg
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		return res, 0, ""
	}
	if len(params) >= 3 && strings.TrimSpace(string(params[2])) != "null" {
		var hexStr string
		if err := json.Unmarshal(params[2], &hexStr); err == nil {
			hexStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexStr), "0x"))
			if raw, err := hex.DecodeString(hexStr); err == nil && len(raw) > 0 {
				if fr, ok := m["feerate"].(float64); ok && fr > 0 {
					baseFee := fr * 250.0 / 1e8
					scaled := baseFee * float64(len(raw)) / 250.0
					m["feerate"] = scaled
					m["fee_rate"] = scaled
				}
			}
		}
	}
	m["dogego_note"] = "subset of Core estimaterawfee; feerate from estimatesmartfee with optional size scaling on unsigned hex"
	return m, 0, ""
}

// execEstimateFee implements the deprecated estimatefee RPC (returns a single number: DOGE/kB).
func execEstimateFee(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 {
		return nil, -8, "estimatefee: nblocks required"
	}
	var v float64
	if err := json.Unmarshal(params[0], &v); err != nil || v < 1 || v > float64(math.MaxInt32) || v != float64(int64(v)) {
		return nil, -8, "estimatefee: invalid nblocks"
	}
	if int(v) == 1 {
		return -1.0, 0, ""
	}
	if paths == nil || (paths.FeeFilter == nil && paths.MempoolFeeEstimate == nil && paths.ConfirmedFeeEstimate == nil) {
		return -1.0, 0, ""
	}
	rate, _, _ := smartFeeKoinuPerKB(paths, int(v), true)
	if rate == 0 {
		return -1.0, 0, ""
	}
	return float64(rate) / 1e8, 0, ""
}

// execEstimatePriority implements the deprecated estimatepriority RPC (Core mining.cpp).
// DogeGo has no coin-age priority market; returns -1 like an empty mempool estimate.
func execEstimatePriority(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 {
		return nil, -8, "estimatepriority: nblocks required"
	}
	var v float64
	if err := json.Unmarshal(params[0], &v); err != nil || v < 1 || v > float64(math.MaxInt32) || v != float64(int64(v)) {
		return nil, -8, "estimatepriority: invalid nblocks"
	}
	_ = paths
	return -1.0, 0, ""
}

// execEstimateSmartPriority implements the deprecated estimatesmartpriority RPC (Core CBlockPolicyEstimator::estimateSmartPriority).
func execEstimateSmartPriority(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	nblocks := 6
	if len(params) >= 1 {
		var v float64
		if err := json.Unmarshal(params[0], &v); err == nil && v >= 1 && v <= 1008 && v == float64(int64(v)) {
			nblocks = int(v)
		}
	}
	priority := -1.0
	if minRelayFeeFromPaths(paths) > 0 {
		priority = consensus.InfPriority
	}
	return map[string]interface{}{
		"priority": priority,
		"blocks":   nblocks,
	}, 0, ""
}
