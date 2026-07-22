// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"path/filepath"
	"strings"
)

// connectErrNeedsTxIndexRepair reports connect failures that often clear after sparse/lag txindex repair.
func connectErrNeedsTxIndexRepair(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "missing funding height") ||
		strings.Contains(s, "funding height:")
}

func blockStoreChainDir(bs *BlockStoreCtx) string {
	if bs == nil {
		return ""
	}
	if bs.Raw != nil {
		return filepath.Dir(bs.Raw.Dir())
	}
	if bs.TxIndex != nil {
		// indexes/tx → chain datadir
		return filepath.Dir(filepath.Dir(bs.TxIndex.RootDir()))
	}
	return ""
}

// maybeRepairTxIndexOnConnectErr runs sparse/lag txindex repair when connect hits funding-height gaps.
func maybeRepairTxIndexOnConnectErr(bs *BlockStoreCtx, err error) {
	if !connectErrNeedsTxIndexRepair(err) {
		return
	}
	chainDir := blockStoreChainDir(bs)
	if chainDir == "" {
		return
	}
	maybeRepairTxIndex(chainDir, bs, 1)
}

// isConnectStallErr is the SyncUtxoCache stall after bodies are stored but chainActive lags.
func isConnectStallErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "connect stalled at height")
}

// maybeRepairTxIndexOnConnectStall attempts sparse repair when catch-up reports a connect stall.
func maybeRepairTxIndexOnConnectStall(bs *BlockStoreCtx, err error) {
	if !isConnectStallErr(err) {
		return
	}
	chainDir := blockStoreChainDir(bs)
	if chainDir == "" {
		return
	}
	maybeRepairTxIndex(chainDir, bs, 1)
}
