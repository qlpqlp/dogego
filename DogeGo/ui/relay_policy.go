// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"dogego/config"
	"dogego/consensus"
	"dogego/mempool"
)

// RelayPolicyForAPI builds summary relay/package/standard fields for /api/summary and capabilities.
func RelayPolicyForAPI(ef config.File, pool *mempool.Pool) map[string]any {
	minRelayCfg := ef.MinRelayTxFeeKoinuPerKB
	if minRelayCfg == 0 {
		minRelayCfg = uint64(consensus.DefaultMinRelayTxFeePerKB)
	}
	incrCfg := ef.IncrementalRelayFeeKoinuPerKB
	if incrCfg == 0 {
		incrCfg = uint64(consensus.DefaultIncrementalRelayFeePerKB)
	}
	if pool != nil {
		if v := pool.IncrementalRelayFeePerKB(); v > 0 {
			incrCfg = v
		}
	}
	rollingMin := uint64(0)
	if pool != nil {
		rollingMin = pool.MinRelayFeePerKB()
	}
	effectiveMinRelay := consensus.EffectiveMinRelayFeePerKB(0, rollingMin)
	if effectiveMinRelay < minRelayCfg {
		effectiveMinRelay = minRelayCfg
	}
	maxMempoolMB := ef.MaxMempoolMB
	if maxMempoolMB <= 0 {
		maxMempoolMB = 300
	}
	limits := consensus.MempoolRelayLimitsFromConfig(
		ef.MaxTxFeeKoinu,
		ef.LimitAncestorCount,
		ef.LimitDescendantCount,
		ef.LimitAncestorSizeKB,
		ef.LimitDescendantSizeKB,
	)
	std := standardPolicyFromFile(ef)
	return map[string]any{
		"minrelaytxfee_doge":       koinuPerKBToDOGE(minRelayCfg),
		"incrementalrelayfee_doge": koinuPerKBToDOGE(incrCfg),
		"effective_minrelay_doge":  koinuPerKBToDOGE(effectiveMinRelay),
		"mempool_rolling_min_koinu": rollingMin,
		"maxmempool_mb":            maxMempoolMB,
		"maxorphantx":              ef.MaxOrphanTx,
		"mempoolfullrbf":           ef.MempoolFullRBF,
		"mempoolexpiry_hours":      ef.MempoolExpiryHours,
		"package_policy":           consensus.MempoolPackagePolicyMap(limits),
		"standard_policy":          consensus.StandardPolicyMap(std),
	}
}

func standardPolicyFromFile(f config.File) consensus.StandardPolicy {
	accept := true
	if f.AcceptDataCarrier != nil {
		accept = *f.AcceptDataCarrier
	}
	permit := true
	if f.PermitBareMultisig != nil {
		permit = *f.PermitBareMultisig
	}
	return consensus.StandardPolicyFromNodeConfig(
		f.HardDustLimitKoinu, accept, permit, f.DatacarrierSize)
}
