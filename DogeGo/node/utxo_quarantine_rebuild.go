// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strconv"
	"time"

	"dogego/applog"
	"dogego/store"
)

// rebuildUtxoFromStoredBodiesAfterQuarantine reconnects UTXO from contiguous stored bodies
// after a misaligned utxo.cache was quarantined at startup (faster than re-downloading blocks).
func rebuildUtxoFromStoredBodiesAfterQuarantine(bs *BlockStoreCtx, utxo *store.UtxoCache) {
	if bs == nil || utxo == nil {
		return
	}
	applog.Line("utxo", "background UTXO rebuild from stored bodies after quarantined snapshot")
	for round := 0; round < 5000; round++ {
		cont := bs.ContiguousRawHeight()
		if cont < 0 {
			time.Sleep(2 * time.Second)
			continue
		}
		tip := utxo.TipHeight()
		if tip >= 0 && tip >= cont-utxoSnapshotBodyMargin {
			applog.Line("utxo", "UTXO rebuild complete through height "+strconv.FormatInt(tip, 10))
			return
		}
		if ConnectCatchUpLag(bs, utxo) >= 2048 {
			runConnectCatchUpStartupBurst(bs, utxo)
		} else if err := bs.SyncUtxoCacheBounded(256); err != nil {
			applog.Line("utxo", "quarantine rebuild: "+err.Error())
			time.Sleep(3 * time.Second)
			continue
		}
		time.Sleep(50 * time.Millisecond)
	}
	applog.Line("utxo", "quarantine UTXO rebuild stopped (contiguous "+strconv.FormatInt(bs.ContiguousRawHeight(), 10)+")")
}
