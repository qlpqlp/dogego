// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "sync/atomic"

// cmpctRelayMetrics tracks BIP152 compact-block relay activity for operator probes.
type cmpctRelayMetrics struct {
	In                atomic.Uint64
	MempoolHit        atomic.Uint64
	GetBlockTxnOut    atomic.Uint64
	BlockTxnIn        atomic.Uint64
	ReconstructOK     atomic.Uint64
	ReconstructFail   atomic.Uint64
	AnnouncedOut      atomic.Uint64
	ServedGetData     atomic.Uint64
	FallbackFullBlock atomic.Uint64
	BlockTxnServed    atomic.Uint64
	ReconstructFallback atomic.Uint64 // inbound reconstruct fail → getdata MSG_BLOCK
}

var cmpctMetrics cmpctRelayMetrics

func annotateCmpctRelayMetrics(out map[string]any) {
	if out == nil {
		return
	}
	out["dogego_cmpct_in"] = cmpctMetrics.In.Load()
	out["dogego_cmpct_mempool_hit"] = cmpctMetrics.MempoolHit.Load()
	out["dogego_cmpct_getblocktxn_out"] = cmpctMetrics.GetBlockTxnOut.Load()
	out["dogego_cmpct_blocktxn_in"] = cmpctMetrics.BlockTxnIn.Load()
	out["dogego_cmpct_reconstruct_ok"] = cmpctMetrics.ReconstructOK.Load()
	out["dogego_cmpct_reconstruct_fail"] = cmpctMetrics.ReconstructFail.Load()
	out["dogego_cmpct_announced_out"] = cmpctMetrics.AnnouncedOut.Load()
	out["dogego_cmpct_served_getdata"] = cmpctMetrics.ServedGetData.Load()
	out["dogego_cmpct_fallback_full_block"] = cmpctMetrics.FallbackFullBlock.Load()
	out["dogego_cmpct_blocktxn_served"] = cmpctMetrics.BlockTxnServed.Load()
	out["dogego_cmpct_reconstruct_fallback_getdata"] = cmpctMetrics.ReconstructFallback.Load()
}
