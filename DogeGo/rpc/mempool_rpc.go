// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"
	"time"

	"dogego/consensus"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

func mempoolEntryJSON(e mempool.MemPoolVerboseEntry, report consensus.PackageFeeReport, pkg mempool.PackageStats, bip125 bool, feeDeltaKoinu int64) map[string]interface{} {
	deps := make([]interface{}, len(e.Depends))
	for i, d := range e.Depends {
		deps[i] = d
	}
	weight := 4 * e.VSize
	baseFee := float64(report.BaseFeeKoinu) / 1e8
	modFee := baseFee + float64(feeDeltaKoinu)/1e8
	ancFee := float64(report.AncestorFeeKoinu) / 1e8
	descFee := float64(report.DescendantFeeKoinu) / 1e8
	effRate := float64(report.EffectiveRatePerKB) / 1e8
	ancRate := effRate
	descRate := baseFee * 1000 / float64(max(e.VSize, 1)) / 1e8
	if pkg.AncestorSize > 0 {
		ancRate = float64(pkg.AncestorFeesKoinu) * 1000 / float64(pkg.AncestorSize) / 1e8
	}
	if pkg.DescendantSize > 0 {
		descRate = float64(pkg.DescendantFeesKoinu) * 1000 / float64(pkg.DescendantSize) / 1e8
	}
	note := ""
	if report.BaseFeeKoinu == 0 && report.EffectiveRatePerKB == 0 {
		note = "fee unavailable (missing prevouts for fee calculation)"
	}
	entryTime := e.Time
	if entryTime <= 0 {
		entryTime = time.Now().Unix()
	}
	return map[string]interface{}{
		"txid":        e.TxID,
		"wtxid":       e.TxID,
		"size":        e.Size,
		"vsize":       e.VSize,
		"weight":      weight,
		"fee":         baseFee,
		"modifiedfee": modFee,
		"fees": map[string]interface{}{
			"base":              baseFee,
			"modified":          modFee,
			"ancestor":          ancFee,
			"descendant":        descFee,
			"effective-feerate": effRate,
		},
		"fee_rate":           effRate,
		"time":               entryTime,
		"height":               e.Height,
		"startingpriority":     0,
		"currentpriority":      0,
		"descendantcount":    pkg.DescendantCount,
		"descendantsize":     pkg.DescendantSize,
		"descendantfees":     pkg.DescendantFeesKoinu,
		"ancestorcount":      pkg.AncestorCount,
		"ancestorsize":       pkg.AncestorSize,
		"ancestorfees":       pkg.AncestorFeesKoinu,
		"depends":            deps,
		"bip125-replaceable": bip125,
		"unbroadcast":        false,
		"dogego_fee_fields_note": note,
		"dogego_ancestor_fee_rate":   ancRate,
		"dogego_descendant_fee_rate": descRate,
	}
}

func mempoolEntryWithFees(pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore, e mempool.MemPoolVerboseEntry) map[string]interface{} {
	view := mempoolAdmissionView(pool, txIndex, blocks)
	var pkg mempool.PackageStats
	var report consensus.PackageFeeReport
	if view != nil {
		fees := consensus.BuildMempoolFeesKoinu(pool, view)
		sizes, _ := pool.BuildMempoolSizes()
		if st, err := pool.PackageStatsForTxID(e.TxID, fees, sizes); err == nil {
			pkg = st
		}
	}
	feeDelta := pool.FeeDeltaKoinu(e.TxID)
	raw, err := pool.GetRawByTxID(e.TxID)
	if err != nil {
		return mempoolEntryJSON(e, report, pkg, false, feeDelta)
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return mempoolEntryJSON(e, report, pkg, false, feeDelta)
	}
	if view != nil {
		if r, err := consensus.PackageFeeReportForTx(tx, pool, view); err == nil {
			report = r
		}
	}
	return mempoolEntryJSON(e, report, pkg, wire.IsBIP125Replaceable(tx), feeDelta)
}

func execGetRawMempool(pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore, params []json.RawMessage) (result interface{}, errCode int, errMsg string) {
	if pool == nil {
		return nil, -18, "getrawmempool: mempool not available"
	}
	verbose := false
	if len(params) > 0 {
		var v interface{}
		if err := json.Unmarshal(params[0], &v); err != nil {
			return nil, -8, "getrawmempool: bad verbose flag"
		}
		switch t := v.(type) {
		case bool:
			verbose = t
		case float64:
			verbose = t != 0
		default:
			return nil, -8, "getrawmempool: bad verbose flag"
		}
	}
	if !verbose {
		ids, err := pool.RawMemPoolTxIDs()
		if err != nil {
			return nil, -1, err.Error()
		}
		return ids, 0, ""
	}
	entries, err := pool.SortedMemPoolVerbose()
	if err != nil {
		return nil, -1, err.Error()
	}
	// Core getrawmempool verbose=true returns a JSON object keyed by txid (not an array).
	out := make(map[string]interface{}, len(entries))
	for _, e := range entries {
		out[e.TxID] = mempoolEntryWithFees(pool, txIndex, blocks, e)
	}
	return out, 0, ""
}

func execGetMempoolInfo(pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore, minRelayKoinuPerKB uint64, orphanCount int, maxOrphan int, maxMempool int, fullRBF bool, rollingMinKoinu uint64, paths *DataPaths) (result interface{}, errCode int, errMsg string) {
	if pool == nil {
		return nil, -18, "getmempoolinfo: mempool not available"
	}
	n := pool.Count()
	b := pool.TotalBytes()
	effectiveMin := float64(minRelayKoinuPerKB) / 1e8
	configuredMin := float64(consensus.MinRelayTxFeePerKB()) / 1e8
	view := mempoolAdmissionView(pool, txIndex, blocks)
	totalFee := float64(consensus.TotalMempoolFeesKoinu(pool, view)) / 1e8
	res := map[string]interface{}{
		"loaded":              true,
		"size":                n,
		"bytes":               b,
		"usage":               b,
		"maxmempool":          maxMempool,
		"mempoolminfee":       effectiveMin,
		"minrelaytxfee":       configuredMin,
		"incrementalrelayfee": float64(consensus.IncrementalRelayFeePerKB()) / 1e8,
		"mempoolexpiry":       pool.ExpiryHours(),
		"unbroadcastcount":    0,
		"fullrbf":             fullRBF,
		"paused":              pool.Paused(),
		"total_fee":           totalFee,
		"mempool_sequence": int64(pool.ChangeSequence()),
		"dogego_orphans":   orphanCount,
	}
	if maxOrphan > 0 {
		res["dogego_max_orphan_tx"] = maxOrphan
	}
	if orphanCount > 0 {
		res["dogego_orphans_note"] = "transactions waiting for in-mempool or chain parents (P2P orphan pool)"
	}
	if rollingMinKoinu > 0 {
		res["dogego_mempool_rolling_minfee_koinu"] = rollingMinKoinu
		res["dogego_mempool_rolling_minfee"] = float64(rollingMinKoinu) / 1e8
	}
	if totalFee == 0 && n > 0 {
		res["dogego_total_fee_note"] = "total_fee sums per-tx fees when prevouts resolve; may be 0 if prevouts are missing"
	}
	if paths != nil && paths.FeeBucketEstimates != nil {
		if buckets := paths.FeeBucketEstimates(); len(buckets) > 0 {
			res["dogego_fee_buckets"] = buckets
		}
	}
	if paths != nil && paths.FeeBucketMarketStats != nil {
		if stats := paths.FeeBucketMarketStats(); len(stats) > 0 {
			res["dogego_fee_bucket_market"] = stats
		}
	}
	if paths != nil && paths.MempoolConfirmBucketStats != nil {
		if stats := paths.MempoolConfirmBucketStats(); len(stats) > 0 {
			res["dogego_mempool_confirm_buckets"] = stats
		}
	}
	if paths != nil && paths.FeeConfirmStatsBucketMarket != nil {
		if stats := paths.FeeConfirmStatsBucketMarket(); len(stats) > 0 {
			res["dogego_fee_confirm_stats"] = stats
		}
	}
	if paths != nil && paths.MempoolLeftBucketStats != nil {
		if stats := paths.MempoolLeftBucketStats(); len(stats) > 0 {
			res["dogego_mempool_left_buckets"] = stats
		}
	}
	if paths != nil && paths.FeeHistory != nil {
		if n := paths.FeeHistory.PendingMempoolFeeTracks(); n > 0 {
			res["dogego_mempool_fee_pending_tracks"] = n
		}
		if n := paths.FeeHistory.LeftWithoutConfirmCount(); n > 0 {
			res["dogego_mempool_left_without_confirm"] = n
		}
		if n := paths.FeeHistory.ConfirmStatsPendingTracks(); n > 0 {
			res["dogego_fee_confirm_stats_pending"] = n
		}
	}
	if paths != nil && paths.MempoolLimits != nil {
		res["dogego_package_policy"] = consensus.MempoolPackagePolicyMap(paths.MempoolLimits())
	}
	if paths != nil && paths.Standard != nil {
		sp := paths.Standard()
		res["dogego_standard_policy"] = consensus.StandardPolicyMap(sp)
		res["permitbaremultisig"] = sp.AllowBareMultisig
		maxCarrier := sp.MaxDatacarrierBytes
		if maxCarrier <= 0 {
			maxCarrier = consensus.MaxDatacarrierBytes
		}
		res["maxdatacarriersize"] = maxCarrier
	}
	if perc := mempoolFeeratePercentilesDOGE(pool, txIndex, blocks); len(perc) == 5 {
		res["feerate_percentiles"] = perc
	} else {
		res["feerate_percentiles"] = []interface{}{0.0, 0.0, 0.0, 0.0, 0.0}
	}
	return res, 0, ""
}

func mempoolTxNotFoundCode(err error) (int, string) {
	if err == nil {
		return 0, ""
	}
	msg := err.Error()
	if strings.Contains(msg, "transaction not in mempool") {
		return -5, "Transaction not in mempool"
	}
	if strings.Contains(msg, "txid must be") {
		return -8, msg
	}
	return -1, msg
}

func execGetMempoolEntry(pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if pool == nil {
		return nil, -18, "getmempoolentry: mempool not available"
	}
	if len(params) != 1 {
		return nil, -8, "getmempoolentry: txid required"
	}
	var txid string
	if err := json.Unmarshal(params[0], &txid); err != nil {
		return nil, -8, "getmempoolentry: bad txid"
	}
	e, err := pool.MemPoolVerboseEntryForTxID(txid)
	if err != nil {
		code, msg := mempoolTxNotFoundCode(err)
		return nil, code, msg
	}
	return mempoolEntryWithFees(pool, txIndex, blocks, e), 0, ""
}

func execGetMempoolAncestors(pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if pool == nil {
		return nil, -18, "getmempoolancestors: mempool not available"
	}
	if len(params) < 1 || len(params) > 2 {
		return nil, -8, "getmempoolancestors: txid (and optional verbose)"
	}
	verbose := false
	if len(params) > 1 {
		var v interface{}
		if err := json.Unmarshal(params[1], &v); err != nil {
			return nil, -8, "getmempoolancestors: bad verbose flag"
		}
		switch t := v.(type) {
		case bool:
			verbose = t
		case float64:
			verbose = t != 0
		default:
			return nil, -8, "getmempoolancestors: bad verbose flag"
		}
	}
	var txid string
	if err := json.Unmarshal(params[0], &txid); err != nil {
		return nil, -8, "getmempoolancestors: bad txid"
	}
	ids, err := pool.MempoolAncestorTxIDs(txid)
	if err != nil {
		code, msg := mempoolTxNotFoundCode(err)
		return nil, code, msg
	}
	if !verbose {
		arr := make([]interface{}, len(ids))
		for i, id := range ids {
			arr[i] = id
		}
		return arr, 0, ""
	}
	out := make(map[string]interface{})
	for _, id := range ids {
		e, err := pool.MemPoolVerboseEntryForTxID(id)
		if err != nil {
			continue
		}
		out[id] = mempoolEntryWithFees(pool, txIndex, blocks, e)
	}
	return out, 0, ""
}

func execGetMempoolDescendants(pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if pool == nil {
		return nil, -18, "getmempooldescendants: mempool not available"
	}
	if len(params) < 1 || len(params) > 2 {
		return nil, -8, "getmempooldescendants: txid (and optional verbose)"
	}
	verbose := false
	if len(params) > 1 {
		var v interface{}
		if err := json.Unmarshal(params[1], &v); err != nil {
			return nil, -8, "getmempooldescendants: bad verbose flag"
		}
		switch t := v.(type) {
		case bool:
			verbose = t
		case float64:
			verbose = t != 0
		default:
			return nil, -8, "getmempooldescendants: bad verbose flag"
		}
	}
	var txid string
	if err := json.Unmarshal(params[0], &txid); err != nil {
		return nil, -8, "getmempooldescendants: bad txid"
	}
	ids, err := pool.MempoolDescendantTxIDs(txid)
	if err != nil {
		code, msg := mempoolTxNotFoundCode(err)
		return nil, code, msg
	}
	if !verbose {
		arr := make([]interface{}, len(ids))
		for i, id := range ids {
			arr[i] = id
		}
		return arr, 0, ""
	}
	out := make(map[string]interface{})
	for _, id := range ids {
		e, err := pool.MemPoolVerboseEntryForTxID(id)
		if err != nil {
			continue
		}
		out[id] = mempoolEntryWithFees(pool, txIndex, blocks, e)
	}
	return out, 0, ""
}

func mempoolFeesForTx(tx *wire.Tx, pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore) (feeDOGE, feeRate float64) {
	return txFeeDOGE(tx, mempoolAdmissionView(pool, txIndex, blocks))
}
