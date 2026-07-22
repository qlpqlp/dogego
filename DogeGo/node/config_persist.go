// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"dogego/config"
	"dogego/node/dgr"
)

// PersistMaxOutbound writes maxoutbound to dogecoinconf.json (Core setmaxconnections persists nMaxConnections).
func PersistMaxOutbound(savePath string, eff *config.File, max int) error {
	if savePath == "" || eff == nil || max < 8 {
		return nil
	}
	eff.MaxOutbound = max
	return config.Save(savePath, *eff)
}

// PersistDGRRelaySeeds merges discovered DGR operator QUIC addresses into dogecoinconf.json
// relay_seeds (Public relay addresses) without removing operator-edited entries.
func PersistDGRRelaySeeds(savePath string, eff *config.File, seeds []string) error {
	if savePath == "" || eff == nil {
		return nil
	}
	merged := dgr.MergeRelaySeedLists(eff.DogeGoRelayCGNAT.RelaySeeds, seeds, 32)
	eff.DogeGoRelayCGNAT.RelaySeeds = merged
	return config.Save(savePath, *eff)
}
