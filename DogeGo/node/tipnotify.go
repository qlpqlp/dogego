// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"dogego/rpc"
	"dogego/store"
)

// NotifyRPCTip publishes chainActive (UTXO connect tip when wired) to waitfor* RPCs.
func NotifyRPCTip(j *store.HeaderJournal, raw *store.RawBlockStore, utxo *store.UtxoCache, tw *rpc.TipWaiter) {
	NotifyRPCTipWithContiguous(j, raw, utxo, nil, tw)
}

// NotifyRPCTipWithContiguous supplies a contiguous-raw cache when UTXO is absent (headers-only / SPV).
func NotifyRPCTipWithContiguous(j *store.HeaderJournal, raw *store.RawBlockStore, utxo *store.UtxoCache, contiguous func() int64, tw *rpc.TipWaiter) {
	if j == nil || tw == nil {
		return
	}
	var paths *rpc.DataPaths
	if utxo != nil || contiguous != nil {
		paths = &rpc.DataPaths{Utxo: utxo}
		if contiguous != nil {
			paths.ContiguousRawHeight = contiguous
		}
	}
	h, hash, err := rpc.ChainActiveTip(j, raw, paths)
	if err != nil || h < 0 || hash == "" {
		return
	}
	tw.Notify(h, hash)
	rpc.NotifyGBTWake()
}
