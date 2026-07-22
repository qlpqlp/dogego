// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"

	"dogego/chain"
	"dogego/store"
)

// HeaderJournalRecoveryResult reports an operator-triggered header journal rewind.
type HeaderJournalRecoveryResult struct {
	Rewound   bool  `json:"rewound"`
	TipBefore int64 `json:"tip_before"`
	TipAfter  int64 `json:"tip_after"`
}

// RecoverHeaderJournal runs the same local fixes as automatic header sync recovery
// (compressed-period detection + deep rewind one retarget window). Safe while the node is running.
func RecoverHeaderJournal(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, bs *BlockStoreCtx) (HeaderJournalRecoveryResult, error) {
	out := HeaderJournalRecoveryResult{TipBefore: -1, TipAfter: -1}
	if j == nil {
		return out, fmt.Errorf("header journal not available")
	}
	tip, err := j.TipHeight()
	if err != nil {
		return out, err
	}
	out.TipBefore = tip
	ok, recErr := runLocalHeaderJournalRecovery(j, aux, p, bs, headerSyncLastFailure())
	if recErr != nil {
		return out, recErr
	}
	if bs != nil {
		bs.ResetContiguousTip()
	}
	newTip, err := j.TipHeight()
	if err != nil {
		return out, err
	}
	out.TipAfter = newTip
	out.Rewound = ok && newTip < tip
	if !out.Rewound {
		return out, fmt.Errorf("no header journal change (tip still %d)", tip)
	}
	return out, nil
}
