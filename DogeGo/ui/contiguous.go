// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"strings"

	"dogego/rpc"
	"dogego/store"
)

// maxTipForContiguousScan is the header tip above which /api/summary avoids a full
// genesis→tip raw-body scan when no cached height is wired (prevents UI hangs).
const maxTipForContiguousScan = 50_000

// contiguousHeightForAPI returns cached contiguous raw height when ContiguousRawHeight is set.
func contiguousHeightForAPI(cfg StartConfig) int64 {
	if cfg.ContiguousRawHeight != nil {
		return cfg.ContiguousRawHeight()
	}
	if cfg.Journal == nil || cfg.RawBlocks == nil {
		return -1
	}
	tip, _, err := journalTipForDashboard(cfg.Journal)
	if err != nil || tip < 0 || tip > maxTipForContiguousScan {
		return -1
	}
	if ch, err := store.ContiguousRawBodyHeight(cfg.Journal, cfg.RawBlocks); err == nil {
		return ch
	}
	return -1
}

// chainActiveHeightForAPI returns Core chainActive (UTXO/connect tip), not stored bodies ahead of ConnectBlock.
func chainActiveHeightForAPI(cfg StartConfig, headerTip int64) int64 {
	if cfg.ChainIBDSync != nil {
		snap := cfg.ChainIBDSync()
		if snap.Blocks >= 0 {
			return snap.Blocks
		}
	}
	if cfg.RawBlocks == nil {
		if headerTip >= 0 {
			return headerTip
		}
		return 0
	}
	return rpc.ActiveChainBlockHeight(cfg.Journal, cfg.RawBlocks)
}

// chainStatsHints returns (chainActive, storedBodies) for BuildChainStats / live cache.
func chainStatsHints(cfg StartConfig) (chainActive, storedBodies int64) {
	tip := int64(-1)
	if cfg.Journal != nil {
		tip, _, _ = journalTipForDashboard(cfg.Journal)
	}
	return chainActiveHeightForAPI(cfg, tip), contiguousHeightForAPI(cfg)
}

// rpcChainFromUISlug maps UI network slug to JSON-RPC chain name (main / test).
func rpcChainFromUISlug(slug string) string {
	switch strings.ToLower(strings.TrimSpace(slug)) {
	case "main", "mainnet":
		return "main"
	default:
		return "test"
	}
}
