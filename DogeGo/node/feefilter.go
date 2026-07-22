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
	"dogego/wire"
	"fmt"
)

// LocalMinRelayFeePerKB is this node's advertised minimum relay feerate (default + rolling mempool floor).
func LocalMinRelayFeePerKB(pool *mempool.Pool) uint64 {
	var rolling uint64
	if pool != nil {
		rolling = pool.MinRelayFeePerKB()
	}
	return consensus.EffectiveMinRelayFeePerKB(0, rolling)
}

// SendFeeFilter advertises our min relay feerate to one peer (always sent on new relay connect).
func SendFeeFilter(mw *MsgWriter, pool *mempool.Pool) error {
	if mw == nil {
		return nil
	}
	rate := LocalMinRelayFeePerKB(pool)
	return mw.Write("feefilter", wire.EncodeFeeFilterPayload(rate))
}

// MaybeBroadcastFeeFilter sends feefilter when the local rate changed (Core BIP133).
func MaybeBroadcastFeeFilter(mw *MsgWriter, pool *mempool.Pool, lastSent *uint64) error {
	if mw == nil {
		return nil
	}
	rate := LocalMinRelayFeePerKB(pool)
	if lastSent != nil && *lastSent == rate {
		return nil
	}
	if err := mw.Write("feefilter", wire.EncodeFeeFilterPayload(rate)); err != nil {
		return err
	}
	if lastSent != nil {
		*lastSent = rate
	}
	applog.Line("net", fmt.Sprintf("feefilter sent: min relay fee rate %d koinu/kB", rate))
	return nil
}
