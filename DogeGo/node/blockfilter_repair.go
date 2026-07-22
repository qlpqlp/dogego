// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strconv"

	"dogego/applog"
	"dogego/store"
)

func maybeRepairBlockFilters(j *store.HeaderJournal, chainDir string, minRawBlocks int, index store.BlockFilterIndexer) {
	if j == nil || chainDir == "" || minRawBlocks <= 0 || index == nil {
		return
	}
	rep, ran, err := store.RepairBlockFiltersIfLag(j, chainDir, minRawBlocks, index)
	if err != nil {
		applog.Line("block", "block filter repair: "+err.Error())
		return
	}
	if ran {
		applog.Line("block", "block filter repair: indexed "+strconv.Itoa(rep.BlocksIndexed)+" block(s)")
	}
}
