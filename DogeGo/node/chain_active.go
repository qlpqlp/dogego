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

// ChainActiveHeight returns Core chainActive (UTXO connect tip when wired, else contiguous bodies / headers).
func ChainActiveHeight(j *store.HeaderJournal, raw *store.RawBlockStore, utxo *store.UtxoCache, contiguous func() int64) int64 {
	if j == nil {
		return -1
	}
	var paths *rpc.DataPaths
	if utxo != nil || contiguous != nil {
		paths = &rpc.DataPaths{Utxo: utxo}
		if contiguous != nil {
			paths.ContiguousRawHeight = contiguous
		}
	}
	h := rpc.ActiveChainBlockHeight(j, raw, paths)
	if h >= 0 {
		return h
	}
	return -1
}
