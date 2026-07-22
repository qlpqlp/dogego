// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"dogego/chain"
	"dogego/consensus"
)

// BuildSoftforksForTip returns Core-shaped softforks and bip9_softforks for getblockchaininfo.
// When j is non-nil, bip9 entries are computed with the BIP9 version-bits state machine (Core versionbits.cpp).
func BuildSoftforksForTip(j consensus.HeaderChain, tip int64, net chain.Network) ([]interface{}, map[string]interface{}) {
	if tip < 0 {
		tip = 0
	}
	dc := consensus.LookupConsensus(net, tip)
	soft := []interface{}{
		softForkDesc("bip34", 2, dc.BIP34Height, tip),
		softForkDesc("bip66", 3, dc.BIP66Height, tip),
		softForkDesc("bip65", 4, dc.BIP65Height, tip),
	}
	bip9 := map[string]interface{}{}
	p := consensus.BIP9ParamsForNetwork(net)
	for _, dep := range p.Deployments {
		bip9[dep.Name] = bip9DeploymentDesc(j, net, dep, p, tip, dc.CSVHeight)
	}
	return soft, bip9
}

func softForkDesc(id string, version, activationHeight int, tip int64) map[string]interface{} {
	active := tip >= int64(activationHeight)
	return map[string]interface{}{
		"id":      id,
		"version": version,
		"reject": map[string]interface{}{
			"status": active,
		},
	}
}

func bip9DeploymentDesc(j consensus.HeaderChain, net chain.Network, dep consensus.BIP9Deployment, p consensus.BIP9Params, tip int64, csvHeight int) map[string]interface{} {
	if dep.Timeout == 0 {
		return map[string]interface{}{
			"status":    consensus.ThresholdDefined.String(),
			"bit":       dep.Bit,
			"startTime": dep.StartTime,
			"timeout":   dep.Timeout,
			"since":     int64(0),
		}
	}
	if j != nil {
		r, err := consensus.EvaluateBIP9AtHeight(j, tip, net, dep, p.Period, p.Threshold)
		if err == nil {
			since := r.Since
			if dep.Name == "csv" && r.Status == consensus.ThresholdActive && int64(csvHeight) > since {
				since = int64(csvHeight)
			}
			out := map[string]interface{}{
				"status":    r.Status.String(),
				"bit":       r.Bit,
				"startTime": r.StartTime,
				"timeout":   r.Timeout,
				"since":     since,
			}
			if stats := bip9Statistics(j, net, dep, p, tip, r.Status); stats != nil {
				out["statistics"] = stats
			}
			return out
		}
	}
	if dep.Name == "csv" {
		return bip9CSVHeightFallback(csvHeight, tip, dep)
	}
	return map[string]interface{}{
		"status":    consensus.ThresholdDefined.String(),
		"bit":       dep.Bit,
		"startTime": dep.StartTime,
		"timeout":   dep.Timeout,
		"since":     int64(0),
	}
}

func bip9CSVHeightFallback(csvHeight int, tip int64, dep consensus.BIP9Deployment) map[string]interface{} {
	status := consensus.ThresholdDefined.String()
	since := int64(0)
	if tip >= int64(csvHeight) {
		status = consensus.ThresholdActive.String()
		since = int64(csvHeight)
	}
	return map[string]interface{}{
		"status":    status,
		"bit":       dep.Bit,
		"startTime": dep.StartTime,
		"timeout":   dep.Timeout,
		"since":     since,
	}
}
