// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"

	"dogego/chain"
	"dogego/consensus"
	"dogego/pow"
	"dogego/store"
)

// cappedHeaderJournal reports TipHeight() as min(real tip, cap) for BIP9 evaluation at a block.
type cappedHeaderJournal struct {
	HeaderJournal
	cap int64
}

func (c cappedHeaderJournal) TipHeight() (int64, error) {
	tip, err := c.HeaderJournal.TipHeight()
	if err != nil {
		return -1, err
	}
	if tip > c.cap {
		return c.cap, nil
	}
	return tip, nil
}

// execGetDeploymentInfo returns Core-shaped deployment state at tip or optional block hash.
func execGetDeploymentInfo(j HeaderJournal, raw *store.RawBlockStore, paths *DataPaths, chainName string, params []json.RawMessage) (interface{}, int, string) {
	if j == nil {
		return nil, -1, "getdeploymentinfo: header journal not available"
	}
	height, hash, code, msg := resolveDeploymentQueryHeight(j, raw, paths, params)
	if code != 0 {
		return nil, code, msg
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -8, err.Error()
	}
	dc := consensus.LookupConsensus(net, height)
	p := consensus.BIP9ParamsForNetwork(net)
	jAt := cappedHeaderJournal{HeaderJournal: j, cap: height}
	deployments := map[string]interface{}{
		"bip34": buriedDeployment("bip34", dc.BIP34Height, height),
		"bip66": buriedDeployment("bip66", dc.BIP66Height, height),
		"bip65": buriedDeployment("bip65", dc.BIP65Height, height),
	}
	for _, dep := range p.Deployments {
		deployments[dep.Name] = bip9DeploymentInfo(jAt, net, dep, p, height, dc.CSVHeight)
	}
	return map[string]interface{}{
		"hash":        hash,
		"height":      height,
		"deployments": deployments,
	}, 0, ""
}

func resolveDeploymentQueryHeight(j HeaderJournal, raw *store.RawBlockStore, paths *DataPaths, params []json.RawMessage) (height int64, hash string, code int, msg string) {
	height, _, _ = activeChainFromJournal(j, raw, paths)
	h80, err := j.ReadHeaderAt(height)
	if err != nil {
		return 0, "", -1, err.Error()
	}
	hash = pow.BlockHashHex(h80)
	if len(params) == 0 || strings.TrimSpace(string(params[0])) == "null" {
		return height, hash, 0, ""
	}
	blockhash, c, m := parseOneBlockHashParam(params, "getdeploymentinfo")
	if c != 0 {
		return 0, "", c, m
	}
	h, err := j.HeightByDisplayHash(blockhash)
	if err != nil {
		return 0, "", -5, "Block not found"
	}
	h80, err = j.ReadHeaderAt(h)
	if err != nil {
		return 0, "", -1, err.Error()
	}
	return h, pow.BlockHashHex(h80), 0, ""
}

func buriedDeployment(name string, activation int, atHeight int64) map[string]interface{} {
	active := atHeight >= int64(activation)
	out := map[string]interface{}{
		"type":   "buried",
		"active": active,
	}
	if active {
		out["height"] = activation
	}
	_ = name
	return out
}

func bip9DeploymentInfo(j consensus.HeaderChain, net chain.Network, dep consensus.BIP9Deployment, p consensus.BIP9Params, atHeight int64, csvHeight int) map[string]interface{} {
	active := false
	var actHeight interface{}
	bip9 := map[string]interface{}{
		"bit":       dep.Bit,
		"start_time": dep.StartTime,
		"timeout":   dep.Timeout,
		"status":    consensus.ThresholdDefined.String(),
		"since":     int64(0),
	}
	if dep.Timeout != 0 && j != nil {
		r, err := consensus.EvaluateBIP9AtHeight(j, atHeight, net, dep, p.Period, p.Threshold)
		if err == nil {
			bip9["status"] = r.Status.String()
			bip9["since"] = r.Since
			if dep.Name == "csv" && r.Status == consensus.ThresholdActive && int64(csvHeight) > r.Since {
				bip9["since"] = int64(csvHeight)
			}
			if stats := bip9Statistics(j, net, dep, p, atHeight, r.Status); stats != nil {
				bip9["statistics"] = stats
			}
			active = r.Status == consensus.ThresholdActive
			if active {
				actHeight = bip9["since"]
			}
		}
	} else if dep.Name == "csv" && atHeight >= int64(csvHeight) {
		bip9["status"] = consensus.ThresholdActive.String()
		bip9["since"] = int64(csvHeight)
		active = true
		actHeight = csvHeight
	}
	out := map[string]interface{}{
		"type":   "bip9",
		"active": active,
		"bip9":   bip9,
	}
	if active && actHeight != nil {
		out["height"] = actHeight
	}
	return out
}
