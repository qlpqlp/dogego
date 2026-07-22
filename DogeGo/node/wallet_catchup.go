// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"
	"strconv"

	"dogego/applog"
	"dogego/rpc"
	"dogego/store"
	"dogego/wallet"
)

// StartWalletCatchUpRescan runs SyncUtxo and an incremental block scan when the wallet
// has fallen behind the contiguous raw chain tip (Core rescan-on-open analogue).
func StartWalletCatchUpRescan(ctx context.Context, paths *rpc.DataPaths, j *store.HeaderJournal, raw *store.RawBlockStore, disk *wallet.Disk) {
	if paths == nil || disk == nil || j == nil || raw == nil {
		return
	}
	go func() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		cont, err := store.ContiguousRawBodyHeight(j, raw)
		if err != nil || cont < 0 {
			return
		}
		start := disk.MaxScannedBlockHeight() + 1
		if start < 0 {
			start = 0
		}
		if start > cont {
			max := disk.MaxScannedBlockHeight()
			applog.Line("wallet", "catch-up skipped: indexed through "+strconv.FormatInt(max, 10)+
				", contiguous tip "+strconv.FormatInt(cont, 10))
			return
		}
		if paths.SyncUtxo != nil && !rpc.WalletRescanUtxoSynced(paths, j, raw) {
			if err := paths.SyncUtxo(); err != nil {
				applog.Line("wallet", "catch-up SyncUtxo: "+err.Error())
				return
			}
		}
		if paths.WalletRescanBlocks == nil {
			return
		}
		applog.Line("wallet", fmt.Sprintf("incremental rescan from height %d through %d", start, cont))
		if err := paths.WalletRescanBlocks(start); err != nil {
			applog.Line("wallet", "catch-up rescan: "+err.Error())
			return
		}
		rpc.RefreshWalletUtxoCache(paths, disk.TrackedScripts())
	}()
}
