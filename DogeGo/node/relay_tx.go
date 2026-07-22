// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"dogego/applog"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
	"fmt"
)

// RelayTxToPeer advertises a tx via inv(MSG_TX) when it meets the peer's feefilter (Core inv relay).
func RelayTxToPeer(mw *MsgWriter, raw []byte, peerFeeFilter uint64, pool *mempool.Pool, txIx *store.TxIndex, rawBlocks *store.RawBlockStore) error {
	if mw == nil || len(raw) == 0 {
		return nil
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return err
	}
	view := consensus.AdmissionPrevOutView(pool, txIx, rawBlocks)
	rate, ok := consensus.TxFeeRateKoinuPerKB(tx, raw, view)
	if !ok {
		applog.Line("mempool", "tx relay skipped: fee rate unknown (missing prevouts)")
		return nil
	}
	if !consensus.MeetsPeerFeeFilter(rate, peerFeeFilter) {
		applog.Line("mempool", fmt.Sprintf("tx relay skipped: feerate %d < peer feefilter %d koinu/kB", rate, peerFeeFilter))
		return nil
	}
	body, err := wire.EncodeInvPayload([]wire.InvEntry{{Type: wire.InvTypeTx, Hash: tx.TxHash()}})
	if err != nil {
		return err
	}
	return mw.Write("inv", body)
}
