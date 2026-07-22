// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"math/big"

	"dogego/rpc"
	"dogego/store"
)

// ChainIBDSnapshot reports Core-shaped IBD state for dashboard / P2P snapshots.
func ChainIBDSnapshot(j *store.HeaderJournal, chainRPCName string, raw *store.RawBlockStore, paths *rpc.DataPaths) rpc.ChainIBDSnapshot {
	if j == nil {
		return rpc.ChainIBDSnapshot{}
	}
	return rpc.ComputeChainIBDSnapshot(j, chainRPCName, raw, paths)
}

// snapshotIBDPaths returns RPC paths when the server is up, else a minimal view for UI/P2P IBD.
func snapshotIBDPaths(rpcPaths *rpc.DataPaths, maxTipAgeSec int64, medianOffset func() int32) *rpc.DataPaths {
	if rpcPaths != nil {
		return rpcPaths
	}
	p := &rpc.DataPaths{MaxTipAgeSec: maxTipAgeSec}
	if medianOffset != nil {
		p.MedianPeerTimeOffset = medianOffset
	}
	return p
}

// mergeUtxoIntoIBDPaths wires the in-memory UTXO cache for chainActive height in IBD snapshots.
func mergeUtxoIntoIBDPaths(p *rpc.DataPaths, utxo *store.UtxoCache) *rpc.DataPaths {
	if utxo == nil {
		return p
	}
	if p == nil {
		p = &rpc.DataPaths{}
	}
	if p.Utxo == nil {
		p.Utxo = utxo
	}
	return p
}

// mergeContiguousHeightIntoIBDPaths ensures IBD/RPC helpers use the block-store cache instead of scanning rawblocks.
func mergeContiguousHeightIntoIBDPaths(p *rpc.DataPaths, contiguous func() int64) *rpc.DataPaths {
	if contiguous == nil {
		return p
	}
	if p == nil {
		p = &rpc.DataPaths{}
	}
	if p.ContiguousRawHeight == nil {
		p.ContiguousRawHeight = contiguous
	}
	return p
}

// mergeChainWorkIntoIBDPaths wires the incremental chain-work cache for dashboard / getblockchaininfo.
func mergeChainWorkIntoIBDPaths(p *rpc.DataPaths, j *store.HeaderJournal, cw *ChainWorkCache) *rpc.DataPaths {
	if cw == nil {
		return p
	}
	if p == nil {
		p = &rpc.DataPaths{}
	}
	if p.CumulativeChainWork == nil {
		p.CumulativeChainWork = func(through int64) (*big.Int, bool) {
			return cw.LookupThrough(j, through)
		}
	}
	if p.ChainWorkCacheReady == nil {
		p.ChainWorkCacheReady = cw.Ready
	}
	return p
}

// mergeCoreIBDIntoProgress sets idle_full from Core initialblockdownload when paths are wired.
func mergeCoreIBDIntoProgress(prog map[string]interface{}, snap rpc.ChainIBDSnapshot) {
	if prog == nil {
		return
	}
	prog["idle_full"] = !snap.IBD
	prog["initialblockdownload"] = snap.IBD
	prog["verification_progress"] = snap.VerificationProgress
}
