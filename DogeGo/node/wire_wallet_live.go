// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strconv"

	"dogego/applog"
	"dogego/rpc"
	"dogego/store"
	"dogego/wallet"
)

func wireWalletLiveIndex(bs *BlockStoreCtx, paths *rpc.DataPaths, disk *wallet.Disk, j *store.HeaderJournal, raw *store.RawBlockStore, pkhVer, shVer byte) {
	if bs == nil || disk == nil {
		return
	}
	bs.AppendOnChainActiveAdvance(func(h int64) {
		if err := disk.IndexConnectedBlock(j, raw, pkhVer, shVer, h); err != nil {
			applog.Line("wallet", "live index height "+strconv.FormatInt(h, 10)+": "+err.Error())
		}
	})
}
