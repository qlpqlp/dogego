// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"

	"dogego/chain"
)

// NextBlockBits returns nBits for a candidate block at nextHeight with the given block time.
// j may be *store.HeaderJournal or any TipHeight + ReadHeaderAt store (RPC HeaderJournal).
func NextBlockBits(j headerTimeBitsStore, net chain.Network, nextHeight int64, blockTime uint32) (uint32, error) {
	if j == nil {
		return 0, fmt.Errorf("nil header journal")
	}
	tip, err := j.TipHeight()
	if err != nil {
		return 0, err
	}
	v := &batchView{j: j, tip0: tip}
	prevH := nextHeight - 1
	return getNextWorkRequired(v, prevH, blockTime, LookupConsensus(net, prevH))
}
