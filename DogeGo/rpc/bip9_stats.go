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

func bip9Statistics(j consensus.HeaderChain, net chain.Network, dep consensus.BIP9Deployment, p consensus.BIP9Params, tip int64, status consensus.ThresholdState) interface{} {
	if status != consensus.ThresholdStarted || j == nil || dep.Timeout == 0 {
		return nil
	}
	st, err := consensus.BIP9PeriodStatsAt(j, tip, dep, p.Period, p.Threshold)
	if err != nil {
		return nil
	}
	return map[string]interface{}{
		"period":    st.Period,
		"threshold": st.Threshold,
		"elapsed":   st.Elapsed,
		"count":     st.Count,
		"possible":  st.Possible,
	}
}
