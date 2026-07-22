// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"dogego/consensus"
	"dogego/store"
)

func blockStatsPrevOutView(parentHeight int64, utxo *store.UtxoCache, txIndex *store.TxIndex, raw *store.RawBlockStore) consensus.PrevOutView {
	var views []consensus.PrevOutView
	if utxo != nil && parentHeight >= 0 && utxo.TipHeight() == parentHeight {
		views = append(views, consensus.UtxoPrevOutView{Source: utxo})
	}
	if txIndex != nil && raw != nil {
		views = append(views, &consensus.ChainPrevOutView{Index: txIndex, Raw: raw})
	}
	switch len(views) {
	case 0:
		return nil
	case 1:
		return views[0]
	default:
		return consensus.MultiPrevOutView(views)
	}
}
