// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"fmt"

	"dogego/config"
	"dogego/consensus"
	"dogego/mempool"
)

func koinuPerKBToDOGE(k uint64) float64 {
	if k == 0 {
		return 0
	}
	return float64(k) / 1e8
}

// MempoolDetailForAPI returns a Core getmempoolinfo-shaped subset plus a capped list of txs (verbose rows).
func MempoolDetailForAPI(pool *mempool.Pool, limit int, conf config.File, orphanCount func() int) map[string]any {
	if pool == nil {
		return map[string]any{
			"loaded":      false,
			"dogego_note": "Mempool not available in this process",
		}
	}
	if limit <= 0 || limit > 500 {
		limit = 120
	}
	entries, err := pool.SortedMemPoolVerbose()
	if err != nil {
		return map[string]any{"loaded": true, "error": err.Error()}
	}
	n := len(entries)
	if n > limit {
		entries = entries[:limit]
	}
	rows := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, map[string]any{
			"txid":    e.TxID,
			"size":    e.Size,
			"vsize":   e.VSize,
			"depends": e.Depends,
		})
	}
	minCfg := conf.MinRelayTxFeeKoinuPerKB
	if minCfg == 0 {
		minCfg = uint64(consensus.DefaultMinRelayTxFeePerKB)
	}
	incrCfg := conf.IncrementalRelayFeeKoinuPerKB
	if incrCfg == 0 {
		incrCfg = pool.IncrementalRelayFeePerKB()
	}
	rolling := pool.MinRelayFeePerKB()
	configuredMin := uint64(consensus.MinRelayTxFeePerKB())
	effectiveMin := consensus.EffectiveMinRelayFeePerKB(0, rolling)
	if effectiveMin < configuredMin {
		effectiveMin = configuredMin
	}
	maxMem := pool.MaxMempoolLimitBytes()
	maxOrphan := conf.MaxOrphanTx
	if maxOrphan <= 0 {
		maxOrphan = 100
	}
	out := map[string]any{
		"loaded":                true,
		"paused":                pool.Paused(),
		"size":                  pool.Count(),
		"bytes":                 pool.TotalBytes(),
		"usage":                 pool.TotalBytes(),
		"maxmempool":            maxMem,
		"dogego_max_transactions": pool.MaxEntries(),
		"dogego_max_orphan_tx":  maxOrphan,
		"dogego_orphan_tx_count": func() int {
			if orphanCount != nil {
				return orphanCount()
			}
			return 0
		}(),
		"mempoolminfee":         koinuPerKBToDOGE(effectiveMin),
		"minrelaytxfee":         koinuPerKBToDOGE(configuredMin),
		"mempoolexpiry":         pool.ExpiryHours(),
		"incrementalrelayfee":   koinuPerKBToDOGE(incrCfg),
		"fullrbf":               conf.MempoolFullRBF,
		"mempool_sequence":      int64(pool.ChangeSequence()),
		"transactions":          rows,
		"transactions_total":    n,
		"transactions_returned": len(rows),
		"dogego_config_minrelay_koinu":       minCfg,
		"dogego_config_incremental_koinu": incrCfg,
		"dogego_mempool_rolling_minfee_koinu": rolling,
	}
	limits := consensus.MempoolRelayLimitsFromConfig(
		conf.MaxTxFeeKoinu,
		conf.LimitAncestorCount,
		conf.LimitDescendantCount,
		conf.LimitAncestorSizeKB,
		conf.LimitDescendantSizeKB,
	)
	out["package_policy"] = consensus.MempoolPackagePolicyMap(limits)
	sp := standardPolicyFromFile(conf)
	out["standard_policy"] = consensus.StandardPolicyMap(sp)
	out["permitbaremultisig"] = sp.AllowBareMultisig
	maxCarrier := sp.MaxDatacarrierBytes
	if maxCarrier <= 0 {
		maxCarrier = consensus.MaxDatacarrierBytes
	}
	out["maxdatacarriersize"] = maxCarrier
	out["feerate_percentiles"] = []float64{0, 0, 0, 0, 0}
	if rolling > 0 && rolling > minCfg {
		out["dogego_note"] = "minrelaytxfee uses max(configured min, mempool rolling floor after eviction); P2P feefilter may raise effective relay further."
	} else {
		out["dogego_note"] = "Relay fees from dogecoinconf.json; live admission also respects P2P feefilter when peers are connected."
	}
	if orphans := out["dogego_orphan_tx_count"].(int); orphans > 0 && pool.Count() == 0 {
		out["dogego_sync_note"] = fmt.Sprintf("%d transaction(s) waiting in the orphan pool (parents not on this node yet - common during initial sync).", orphans)
	}
	return out
}
