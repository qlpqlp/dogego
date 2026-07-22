// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "dogego/chain"

// ScriptFlagsForChain returns height-dependent script verification flags (Core ConnectBlock / mempool consensus flags).
// When journal is non-nil, CSV (BIP68/112) follows BIP9 deployment state at tip; otherwise CSVHeight is used.
func ScriptFlagsForChain(spendHeight int64, net chain.Network, journal HeaderChain) ScriptVerifyFlags {
	dc := LookupConsensus(net, spendHeight)
	var f ScriptVerifyFlags
	if spendHeight >= int64(dc.BIP66Height) {
		f |= ScriptVerifyDERSig
	}
	if spendHeight >= int64(dc.BIP65Height) {
		f |= ScriptVerifyCheckLockTimeVerify
	}
	if csvScriptVerifyActive(spendHeight, net, journal) {
		f |= ScriptVerifyCheckSequenceVerify
	}
	return f
}

func csvScriptVerifyActive(spendHeight int64, net chain.Network, journal HeaderChain) bool {
	if CSVActiveAt(spendHeight, net) {
		return true
	}
	if journal == nil || spendHeight < 0 {
		return false
	}
	p := BIP9ParamsForNetwork(net)
	for i := range p.Deployments {
		if p.Deployments[i].Name != "csv" {
			continue
		}
		dep := p.Deployments[i]
		r, err := EvaluateBIP9AtTip(journal, net, dep, p.Period, p.Threshold)
		if err != nil {
			return false
		}
		return r.Status == ThresholdActive
	}
	return false
}
